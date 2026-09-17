package realtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"go.uber.org/zap"
)

const (
	defaultTicketTTL             = 30 * time.Second
	defaultTicketCleanupInterval = 5 * time.Second
	defaultTicketCapacity        = 10_000
	defaultTicketsPerPrincipal   = 8
	ticketEntropyBytes           = 32
	maximumTicketTTL             = 2 * time.Minute
	maximumTicketCapacity        = 1_000_000
	encodedTicketBytes           = 43
)

type TicketConfig struct {
	TTL                    time.Duration
	CleanupInterval        time.Duration
	Capacity               int
	MaxTicketsPerPrincipal int
}

func DefaultTicketConfig() TicketConfig {
	return TicketConfig{
		TTL:                    defaultTicketTTL,
		CleanupInterval:        defaultTicketCleanupInterval,
		Capacity:               defaultTicketCapacity,
		MaxTicketsPerPrincipal: defaultTicketsPerPrincipal,
	}
}

func normalizeTicketConfig(config TicketConfig) (TicketConfig, error) {
	if config.TTL == 0 {
		config.TTL = defaultTicketTTL
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = defaultTicketCleanupInterval
	}
	if config.Capacity == 0 {
		config.Capacity = defaultTicketCapacity
	}
	if config.MaxTicketsPerPrincipal == 0 {
		config.MaxTicketsPerPrincipal = defaultTicketsPerPrincipal
	}
	if config.TTL <= 0 || config.TTL > maximumTicketTTL ||
		config.CleanupInterval <= 0 || config.CleanupInterval > maximumTicketTTL ||
		config.Capacity <= 0 || config.Capacity > maximumTicketCapacity ||
		config.MaxTicketsPerPrincipal <= 0 ||
		config.MaxTicketsPerPrincipal > config.Capacity {
		return TicketConfig{}, ErrInvalidConfig
	}
	return config, nil
}

// IssuedTicket is the only place the raw ticket exists. Callers must return it
// once and must never log or persist it.
type IssuedTicket struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ticketRecord struct {
	principal authentication.Principal
	expiresAt time.Time
}

// TicketStore holds only SHA-256 lookup keys. Tickets are short-lived,
// capacity-bounded, and atomically removed before a principal is returned.
type TicketStore struct {
	config TicketConfig
	logger *zap.Logger
	now    func() time.Time
	random io.Reader
	life   componentLifecycle

	mu          sync.Mutex
	closed      bool
	tickets     map[[sha256.Size]byte]ticketRecord
	byPrincipal map[string]int
}

func NewTicketStore(config TicketConfig, logger *zap.Logger) (*TicketStore, error) {
	return newTicketStore(config, logger, func() time.Time { return time.Now().UTC() }, rand.Reader)
}

func newTicketStore(
	config TicketConfig,
	logger *zap.Logger,
	now func() time.Time,
	random io.Reader,
) (*TicketStore, error) {
	normalized, err := normalizeTicketConfig(config)
	if err != nil || logger == nil || now == nil || random == nil {
		return nil, ErrInvalidConfig
	}
	return &TicketStore{
		config:      normalized,
		logger:      logger.Named("realtime-ticket"),
		now:         now,
		random:      random,
		life:        newComponentLifecycle(),
		tickets:     make(map[[sha256.Size]byte]ticketRecord, min(normalized.Capacity, 1024)),
		byPrincipal: make(map[string]int),
	}, nil
}

func (*TicketStore) Name() string { return "realtime-ticket-store" }

func (store *TicketStore) Run(ctx context.Context) error {
	if store == nil {
		return ErrInvalidLifecycle
	}
	runContext, err := store.life.start(ctx)
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return nil
		}
		return err
	}
	defer store.life.finish()
	runContext = correlation.EnsureContext(runContext)
	runContext = commonlogger.WithContext(runContext, store.logger)
	log := commonlogger.FromContext(runContext)
	ticker := time.NewTicker(store.config.CleanupInterval)
	defer ticker.Stop()
	log.Info("realtime ticket cleanup started",
		zap.Duration("ticket_ttl", store.config.TTL),
		zap.Int("capacity", store.config.Capacity),
	)
	for {
		select {
		case <-runContext.Done():
			log.Info("realtime ticket cleanup stopped")
			return nil
		case <-ticker.C:
			removed := store.cleanupExpired(store.now())
			if removed > 0 {
				log.Debug("expired realtime tickets removed", zap.Int("count", removed))
			}
		}
	}
}

func (store *TicketStore) Shutdown(ctx context.Context) error {
	if store == nil {
		return ErrInvalidLifecycle
	}
	store.mu.Lock()
	store.closed = true
	clear(store.tickets)
	clear(store.byPrincipal)
	store.mu.Unlock()
	err := store.life.shutdown(ctx)
	return err
}

func (store *TicketStore) Issue(
	ctx context.Context,
	principal authentication.Principal,
) (IssuedTicket, error) {
	if store == nil || ctx == nil {
		return IssuedTicket{}, ErrInvalidConfig
	}
	if store.life.stopped() {
		return IssuedTicket{}, ErrTicketStoreClosed
	}
	logContext := correlation.EnsureContext(ctx)
	now := store.now().UTC()
	if err := validateTicketPrincipal(principal, now); err != nil {
		return IssuedTicket{}, err
	}
	expiresAt := now.Add(store.config.TTL)
	if principal.AuthenticatedUntil.Before(expiresAt) {
		expiresAt = principal.AuthenticatedUntil.UTC()
	}
	if !expiresAt.After(now) {
		return IssuedTicket{}, ErrInvalidPrincipal
	}

	raw := make([]byte, ticketEntropyBytes)
	if _, err := io.ReadFull(store.random, raw); err != nil {
		return IssuedTicket{}, fmt.Errorf("issue realtime ticket: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256(raw)
	for index := range raw {
		raw[index] = 0
	}

	store.mu.Lock()
	if store.closed {
		store.mu.Unlock()
		return IssuedTicket{}, ErrTicketStoreClosed
	}
	if len(store.tickets) >= store.config.Capacity ||
		store.byPrincipal[principal.Subject] >= store.config.MaxTicketsPerPrincipal {
		store.cleanupExpiredLocked(now)
	}
	if len(store.tickets) >= store.config.Capacity ||
		store.byPrincipal[principal.Subject] >= store.config.MaxTicketsPerPrincipal {
		store.mu.Unlock()
		commonlogger.FromContext(logContext).Warn("realtime ticket issuance capacity rejected",
			zap.String("principal_subject", principal.Subject),
		)
		return IssuedTicket{}, ErrTicketCapacity
	}
	if _, collision := store.tickets[digest]; collision {
		store.mu.Unlock()
		return IssuedTicket{}, fmt.Errorf("issue realtime ticket: entropy collision")
	}
	store.tickets[digest] = ticketRecord{principal: principal, expiresAt: expiresAt}
	store.byPrincipal[principal.Subject]++
	store.mu.Unlock()

	commonlogger.FromContext(logContext).Debug("realtime ticket issued",
		zap.String("principal_subject", principal.Subject),
		zap.Time("expires_at", expiresAt),
	)
	return IssuedTicket{Ticket: encoded, ExpiresAt: expiresAt}, nil
}

// IssueFromContext is the protected HTTP integration boundary. The route must
// run behind the live-session authenticator before calling this method.
func (store *TicketStore) IssueFromContext(ctx context.Context) (IssuedTicket, error) {
	if ctx == nil {
		return IssuedTicket{}, ErrInvalidConfig
	}
	principal, exists := authentication.FromContext(ctx)
	if !exists {
		return IssuedTicket{}, ErrInvalidPrincipal
	}
	return store.Issue(ctx, principal)
}

func (store *TicketStore) Consume(
	ctx context.Context,
	rawTicket string,
) (authentication.Principal, error) {
	if store == nil || ctx == nil {
		return authentication.Principal{}, ErrInvalidConfig
	}
	if store.life.stopped() {
		return authentication.Principal{}, ErrTicketStoreClosed
	}
	logContext := correlation.EnsureContext(ctx)
	if len(rawTicket) != encodedTicketBytes {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	decoded, err := base64.RawURLEncoding.DecodeString(rawTicket)
	if err != nil || len(decoded) != ticketEntropyBytes {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	digest := sha256.Sum256(decoded)
	for index := range decoded {
		decoded[index] = 0
	}

	store.mu.Lock()
	if store.closed {
		store.mu.Unlock()
		return authentication.Principal{}, ErrTicketStoreClosed
	}
	record, exists := store.tickets[digest]
	if exists {
		delete(store.tickets, digest)
		store.decrementPrincipalLocked(record.principal.Subject)
	}
	store.mu.Unlock()
	if !exists {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	now := store.now().UTC()
	if !record.expiresAt.After(now) || validateTicketPrincipal(record.principal, now) != nil {
		return authentication.Principal{}, ErrTicketUnavailable
	}
	commonlogger.FromContext(logContext).Debug("realtime ticket consumed",
		zap.String("principal_subject", record.principal.Subject),
	)
	return record.principal, nil
}

func (store *TicketStore) cleanupExpired(now time.Time) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.cleanupExpiredLocked(now.UTC())
}

func (store *TicketStore) cleanupExpiredLocked(now time.Time) int {
	removed := 0
	for digest, record := range store.tickets {
		if record.expiresAt.After(now) {
			continue
		}
		delete(store.tickets, digest)
		store.decrementPrincipalLocked(record.principal.Subject)
		removed++
	}
	return removed
}

func (store *TicketStore) decrementPrincipalLocked(subject string) {
	if store.byPrincipal[subject] <= 1 {
		delete(store.byPrincipal, subject)
		return
	}
	store.byPrincipal[subject]--
}

func validateTicketPrincipal(principal authentication.Principal, now time.Time) error {
	if !principal.Authenticated || principal.Validate(now) != nil {
		return ErrInvalidPrincipal
	}
	userID, err := uuid.Parse(principal.UserID)
	sessionID, sessionErr := uuid.Parse(principal.SessionID)
	tokenID, tokenErr := uuid.Parse(principal.TokenID)
	if err != nil || userID == uuid.Nil || userID.String() != principal.UserID ||
		principal.Subject != principal.UserID || sessionErr != nil ||
		sessionID == uuid.Nil || sessionID.String() != principal.SessionID ||
		tokenErr != nil || tokenID == uuid.Nil || tokenID.String() != principal.TokenID ||
		principal.AuthenticatedAt.IsZero() || principal.SecurityVersion == 0 {
		return ErrInvalidPrincipal
	}
	return nil
}
