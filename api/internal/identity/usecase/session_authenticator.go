package usecase

import (
	"container/list"
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	defaultTouchWorkers         = 2
	defaultTouchQueueCapacity   = 256
	defaultTouchTimeout         = 2 * time.Second
	defaultTouchMinimumInterval = time.Minute
	defaultTouchGateEntries     = 20_000
	maximumTouchWorkers         = 32
	maximumTouchQueueCapacity   = 65_536
	maximumTouchTimeout         = 10 * time.Second
	maximumTouchGateEntries     = 100_000
)

var (
	ErrInvalidSessionAuthenticator = errors.New("identity: invalid session authenticator")
	ErrSessionStateUnavailable     = errors.New("identity: session state unavailable")
	ErrSessionInactive             = errors.New("identity: session is not active")
	ErrUserInactive                = errors.New("identity: user is not active")
	ErrSecurityVersionMismatch     = errors.New("identity: security version mismatch")
	ErrAuthenticationTimeMismatch  = errors.New("identity: authentication time mismatch")
	ErrSessionExpired              = errors.New("identity: session expired")
	ErrAuthenticatorLifecycle      = errors.New("identity: invalid authenticator lifecycle state")
)

// SessionAuthenticatorConfig bounds optional last-seen persistence. The
// authentication decision never waits for these best-effort writes.
type SessionAuthenticatorConfig struct {
	Clock                func() time.Time
	TouchWorkers         int
	TouchQueueCapacity   int
	TouchTimeout         time.Duration
	TouchMinimumInterval time.Duration
	TouchGateEntries     int
}

type authenticatorLifecycleState uint8

const (
	authenticatorNew authenticatorLifecycleState = iota
	authenticatorRunning
	authenticatorStopping
	authenticatorStopped
)

type lastSeenTouch struct {
	sessionID     uuid.UUID
	observedAt    time.Time
	idleExpiresAt time.Time
	cid           string
}

type touchGateEntry struct {
	sessionID uuid.UUID
	at        time.Time
}

type boundedTouchGate struct {
	mu       sync.Mutex
	entries  map[uuid.UUID]*list.Element
	recency  *list.List
	capacity int
	interval time.Duration
}

// SessionAuthenticator wraps cryptographic access-token verification with
// live session/account checks. It is also a lifecycle component that owns a
// fixed last-seen worker pool and bounded queue.
type SessionAuthenticator struct {
	cryptographic ports.AccessTokenVerifier
	repository    ports.SessionRepository
	cache         ports.SessionStateCache
	clock         func() time.Time

	touchQueue   chan lastSeenTouch
	touchWorkers int
	touchTimeout time.Duration
	touchGate    *boundedTouchGate

	lifecycleMu sync.Mutex
	state       authenticatorLifecycleState
	stop        chan struct{}
	done        chan struct{}
	stopOnce    sync.Once
	doneOnce    sync.Once
	workers     sync.WaitGroup
}

// NewSessionAuthenticator validates all dependencies without starting any
// goroutine. Register the returned value with the process lifecycle before the
// HTTP server so touches have one explicit owner.
func NewSessionAuthenticator(
	cryptographic ports.AccessTokenVerifier,
	repository ports.SessionRepository,
	stateCache ports.SessionStateCache,
	config SessionAuthenticatorConfig,
) (*SessionAuthenticator, error) {
	config = normalizeSessionAuthenticatorConfig(config)
	if isNilDependency(cryptographic) || isNilDependency(repository) || isNilDependency(stateCache) ||
		config.Clock == nil || config.TouchWorkers < 1 || config.TouchWorkers > maximumTouchWorkers ||
		config.TouchQueueCapacity < 1 || config.TouchQueueCapacity > maximumTouchQueueCapacity ||
		config.TouchTimeout <= 0 || config.TouchTimeout > maximumTouchTimeout ||
		config.TouchMinimumInterval <= 0 || config.TouchGateEntries < 1 ||
		config.TouchGateEntries > maximumTouchGateEntries {
		return nil, ErrInvalidSessionAuthenticator
	}
	return &SessionAuthenticator{
		cryptographic: cryptographic,
		repository:    repository,
		cache:         stateCache,
		clock:         config.Clock,
		touchQueue:    make(chan lastSeenTouch, config.TouchQueueCapacity),
		touchWorkers:  config.TouchWorkers,
		touchTimeout:  config.TouchTimeout,
		touchGate: &boundedTouchGate{
			entries:  make(map[uuid.UUID]*list.Element),
			recency:  list.New(),
			capacity: config.TouchGateEntries,
			interval: config.TouchMinimumInterval,
		},
		state: authenticatorNew,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}, nil
}

// Verify cryptographically authenticates first and parses UUIDs only from the
// resulting verified principal. This access-session wrapper never accepts a
// refresh purpose; refresh uses its separate opaque-token flow.
func (authenticator *SessionAuthenticator) Verify(
	ctx context.Context,
	raw string,
	expected authentication.TokenPurpose,
) (authentication.Principal, error) {
	if authenticator == nil || ctx == nil {
		return authentication.Anonymous(), ErrInvalidSessionAuthenticator
	}
	if err := ctx.Err(); err != nil {
		return authentication.Anonymous(), err
	}
	if err := expected.Validate(); err != nil {
		return authentication.Anonymous(), err
	}
	if expected != authentication.PurposeAccess {
		return authentication.Anonymous(), authentication.ErrPurposeMismatch
	}

	verified, err := authenticator.cryptographic.Verify(ctx, raw, authentication.PurposeAccess)
	if err != nil {
		logger.FromContext(ctx).Debug("access token cryptographic verification rejected")
		return authentication.Anonymous(), err
	}
	now := authenticator.clock().UTC()
	userID, sessionID, tokenID, err := verifiedPrincipalIDs(verified, now)
	if err != nil {
		return authentication.Anonymous(), err
	}

	state, err := authenticator.cache.Resolve(
		ctx,
		ports.SessionCacheKey{
			SessionID:       sessionID,
			UserID:          userID,
			TokenID:         tokenID.String(),
			SecurityVersion: verified.SecurityVersion,
		},
		verified.AuthenticatedUntil,
		func(loadContext context.Context) (entity.SessionSecurityState, error) {
			return authenticator.repository.GetActiveForAuthentication(loadContext, sessionID, userID)
		},
	)
	if err != nil {
		if ctx.Err() != nil {
			return authentication.Anonymous(), ctx.Err()
		}
		logSessionLookupFailure(ctx, userID, sessionID, err)
		return authentication.Anonymous(), errors.Join(
			authentication.ErrInvalidToken,
			ErrSessionStateUnavailable,
			err,
		)
	}
	boundary, err := validateSessionSecurityState(
		state,
		verified.SecurityVersion,
		verified.AuthenticatedAt,
		userID,
		sessionID,
		now,
	)
	if err != nil {
		logger.FromContext(ctx).Debug(
			"session authentication rejected by live state",
			zap.String("user_id", userID.String()),
			zap.String("session_id", sessionID.String()),
			zap.String("reason", sessionStateReason(err)),
		)
		return authentication.Anonymous(), errors.Join(authentication.ErrInvalidToken, err)
	}
	if verified.AuthenticatedUntil.Before(boundary) {
		boundary = verified.AuthenticatedUntil
	}
	if !boundary.After(now) {
		return authentication.Anonymous(), errors.Join(authentication.ErrInvalidToken, ErrSessionExpired)
	}

	verified.Subject = userID.String()
	verified.UserID = userID.String()
	verified.SessionID = sessionID.String()
	verified.TokenID = tokenID.String()
	verified.Username = state.Username
	// Preserve the time asserted by this credential after proving it does not
	// exceed canonical session state. Never upgrade an older token to a newer
	// step-up: sensitive commands must load LastReauthenticatedAt themselves.
	verified.AuthenticatedUntil = boundary.UTC()
	if err := verified.Validate(now); err != nil {
		return authentication.Anonymous(), errors.Join(authentication.ErrInvalidToken, err)
	}
	authenticator.scheduleLastSeen(ctx, state, now)
	return verified, nil
}

// InvalidateSession synchronously removes local entries. Call it only after
// the durable revocation transaction commits.
func (authenticator *SessionAuthenticator) InvalidateSession(ctx context.Context, sessionID uuid.UUID) error {
	if authenticator == nil {
		return ErrInvalidSessionAuthenticator
	}
	return authenticator.cache.InvalidateSession(ctx, sessionID)
}

// InvalidateUserSessions synchronously removes every cached token for a user.
// Password/status/security-version commands call it only after commit.
func (authenticator *SessionAuthenticator) InvalidateUserSessions(ctx context.Context, userID uuid.UUID) error {
	if authenticator == nil {
		return ErrInvalidSessionAuthenticator
	}
	return authenticator.cache.InvalidateUser(ctx, userID)
}

func (*SessionAuthenticator) Name() string { return "identity-session-authenticator" }

// Run owns the fixed touch workers until the lifecycle is canceled.
func (authenticator *SessionAuthenticator) Run(ctx context.Context) error {
	if authenticator == nil || ctx == nil {
		return ErrInvalidSessionAuthenticator
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	authenticator.lifecycleMu.Lock()
	if authenticator.state != authenticatorNew {
		authenticator.lifecycleMu.Unlock()
		return ErrAuthenticatorLifecycle
	}
	authenticator.state = authenticatorRunning
	for worker := 0; worker < authenticator.touchWorkers; worker++ {
		authenticator.workers.Add(1)
		go authenticator.runTouchWorker()
	}
	authenticator.lifecycleMu.Unlock()

	select {
	case <-ctx.Done():
		authenticator.lifecycleMu.Lock()
		if authenticator.state == authenticatorRunning {
			authenticator.state = authenticatorStopping
		}
		authenticator.lifecycleMu.Unlock()
		authenticator.requestStop()
	case <-authenticator.stop:
	}
	authenticator.workers.Wait()
	authenticator.finishStop()
	return nil
}

// Shutdown is idempotent and honors the caller's lifecycle deadline.
func (authenticator *SessionAuthenticator) Shutdown(ctx context.Context) error {
	if authenticator == nil || ctx == nil {
		return ErrInvalidSessionAuthenticator
	}
	authenticator.lifecycleMu.Lock()
	switch authenticator.state {
	case authenticatorNew:
		authenticator.state = authenticatorStopped
		authenticator.requestStop()
		authenticator.doneOnce.Do(func() { close(authenticator.done) })
	case authenticatorRunning:
		authenticator.state = authenticatorStopping
		authenticator.requestStop()
	case authenticatorStopping, authenticatorStopped:
	default:
		authenticator.lifecycleMu.Unlock()
		return ErrAuthenticatorLifecycle
	}
	done := authenticator.done
	authenticator.lifecycleMu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (authenticator *SessionAuthenticator) scheduleLastSeen(
	ctx context.Context,
	state entity.SessionSecurityState,
	observedAt time.Time,
) {
	authenticator.lifecycleMu.Lock()
	running := authenticator.state == authenticatorRunning
	authenticator.lifecycleMu.Unlock()
	if !running {
		return
	}
	if state.LastSeenAt.IsZero() || observedAt.Sub(state.LastSeenAt) < authenticator.touchGate.interval ||
		!authenticator.touchGate.allow(state.SessionID, observedAt) {
		return
	}
	idleWindow := state.IdleExpiresAt.Sub(state.LastSeenAt)
	absoluteRemaining := state.AbsoluteExpiresAt.Sub(observedAt)
	if idleWindow <= 0 || absoluteRemaining <= 0 {
		return
	}
	idleExpiresAt := state.AbsoluteExpiresAt.UTC()
	if idleWindow < absoluteRemaining {
		idleExpiresAt = observedAt.Add(idleWindow).UTC()
	}
	touch := lastSeenTouch{
		sessionID:     state.SessionID,
		observedAt:    observedAt,
		idleExpiresAt: idleExpiresAt,
		cid:           correlation.FromContext(ctx),
	}
	select {
	case authenticator.touchQueue <- touch:
	default:
		logger.FromContext(ctx).Debug(
			"last-seen touch queue is full",
			zap.String("session_id", state.SessionID.String()),
		)
	}
}

func (authenticator *SessionAuthenticator) runTouchWorker() {
	defer authenticator.workers.Done()
	for {
		// Give shutdown priority. Once stop is closed, each worker performs at
		// most the touch it already owned and never drains an arbitrary backlog.
		select {
		case <-authenticator.stop:
			return
		default:
		}
		select {
		case <-authenticator.stop:
			return
		case touch := <-authenticator.touchQueue:
			authenticator.persistLastSeen(touch)
		}
	}
}

func (authenticator *SessionAuthenticator) persistLastSeen(touch lastSeenTouch) {
	ctx := context.Background()
	if touch.cid != "" {
		ctx = correlation.WithContext(ctx, touch.cid)
	}
	ctx, cancel := context.WithTimeout(ctx, authenticator.touchTimeout)
	defer cancel()
	updated, err := authenticator.repository.TouchLastSeen(
		ctx,
		touch.sessionID,
		touch.observedAt,
		touch.idleExpiresAt,
	)
	if err != nil {
		logger.FromContext(ctx).Debug(
			"failed to persist authentication session last-seen",
			zap.String("session_id", touch.sessionID.String()),
			zap.Error(err),
		)
		return
	}
	if updated {
		authenticator.cache.MarkTouched(touch.sessionID, touch.observedAt, touch.idleExpiresAt)
		return
	}
	// A zero-row conditional update means the cached row lost a race with a
	// revoke, expiry, deletion, or newer observation. Discard it so the next
	// request revalidates canonical state instead of guessing which occurred.
	_ = authenticator.cache.InvalidateSession(ctx, touch.sessionID)
}

func (authenticator *SessionAuthenticator) requestStop() {
	authenticator.stopOnce.Do(func() { close(authenticator.stop) })
}

func (authenticator *SessionAuthenticator) finishStop() {
	authenticator.lifecycleMu.Lock()
	authenticator.state = authenticatorStopped
	authenticator.lifecycleMu.Unlock()
	authenticator.doneOnce.Do(func() { close(authenticator.done) })
}

func (gate *boundedTouchGate) allow(sessionID uuid.UUID, observedAt time.Time) bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if element := gate.entries[sessionID]; element != nil {
		entry := element.Value.(touchGateEntry)
		if observedAt.Sub(entry.at) < gate.interval {
			gate.recency.MoveToFront(element)
			return false
		}
		entry.at = observedAt
		element.Value = entry
		gate.recency.MoveToFront(element)
		return true
	}
	for len(gate.entries) >= gate.capacity {
		oldest := gate.recency.Back()
		if oldest == nil {
			break
		}
		delete(gate.entries, oldest.Value.(touchGateEntry).sessionID)
		gate.recency.Remove(oldest)
	}
	gate.entries[sessionID] = gate.recency.PushFront(touchGateEntry{sessionID: sessionID, at: observedAt})
	return true
}

func verifiedPrincipalIDs(
	principal authentication.Principal,
	now time.Time,
) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	if err := principal.Validate(now); err != nil || !principal.Authenticated ||
		principal.Subject == "" || principal.Subject != principal.UserID ||
		principal.SessionID == "" || principal.TokenID == "" ||
		principal.SecurityVersion == 0 || principal.AuthenticatedAt.IsZero() {
		return uuid.Nil, uuid.Nil, uuid.Nil, authentication.ErrInvalidToken
	}
	userID, err := canonicalUUID(principal.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, authentication.ErrInvalidToken
	}
	sessionID, err := canonicalUUID(principal.SessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, authentication.ErrInvalidToken
	}
	tokenID, err := canonicalUUID(principal.TokenID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, authentication.ErrInvalidToken
	}
	return userID, sessionID, tokenID, nil
}

func validateSessionSecurityState(
	state entity.SessionSecurityState,
	tokenSecurityVersion uint64,
	tokenAuthenticatedAt time.Time,
	userID, sessionID uuid.UUID,
	now time.Time,
) (time.Time, error) {
	if state.SessionID != sessionID || state.UserID != userID {
		return time.Time{}, ErrSessionInactive
	}
	if state.SessionStatus != entity.SessionStatusActive {
		return time.Time{}, ErrSessionInactive
	}
	if state.UserDeleted || state.UserStatus != entity.UserSecurityStatusActive {
		return time.Time{}, ErrUserInactive
	}
	if tokenSecurityVersion == 0 || state.SessionSecurityVersion != tokenSecurityVersion ||
		state.UserSecurityVersion != tokenSecurityVersion {
		return time.Time{}, ErrSecurityVersionMismatch
	}
	if tokenAuthenticatedAt.IsZero() || state.LastReauthenticatedAt.IsZero() ||
		state.LastReauthenticatedAt.After(now) || tokenAuthenticatedAt.After(state.LastReauthenticatedAt) {
		return time.Time{}, ErrAuthenticationTimeMismatch
	}
	if state.LastSeenAt.IsZero() || state.LastSeenAt.After(now) ||
		state.IdleExpiresAt.IsZero() || state.AbsoluteExpiresAt.IsZero() ||
		!state.IdleExpiresAt.After(state.LastSeenAt) ||
		state.IdleExpiresAt.After(state.AbsoluteExpiresAt) ||
		!state.IdleExpiresAt.After(now) || !state.AbsoluteExpiresAt.After(now) {
		return time.Time{}, ErrSessionExpired
	}
	return earliestTime(state.IdleExpiresAt.UTC(), state.AbsoluteExpiresAt.UTC()), nil
}

func canonicalUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, authentication.ErrInvalidToken
	}
	return parsed, nil
}

func sessionStateReason(err error) string {
	switch {
	case errors.Is(err, ErrSessionInactive):
		return "session_inactive"
	case errors.Is(err, ErrUserInactive):
		return "user_inactive"
	case errors.Is(err, ErrSecurityVersionMismatch):
		return "security_version_mismatch"
	case errors.Is(err, ErrAuthenticationTimeMismatch):
		return "authentication_time_mismatch"
	case errors.Is(err, ErrSessionExpired):
		return "session_expired"
	default:
		return "invalid_state"
	}
}

func logSessionLookupFailure(ctx context.Context, userID, sessionID uuid.UUID, err error) {
	fields := []zap.Field{
		zap.String("user_id", userID.String()),
		zap.String("session_id", sessionID.String()),
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) &&
		(applicationError.Code == apperr.CodeDatabaseError || applicationError.Code == apperr.CodeInternalServer) {
		logger.FromContext(ctx).Warn("session authentication state storage failed", append(fields, zap.Error(err))...)
		return
	}
	logger.FromContext(ctx).Debug("session authentication state lookup failed", fields...)
}

func normalizeSessionAuthenticatorConfig(config SessionAuthenticatorConfig) SessionAuthenticatorConfig {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.TouchWorkers == 0 {
		config.TouchWorkers = defaultTouchWorkers
	}
	if config.TouchQueueCapacity == 0 {
		config.TouchQueueCapacity = defaultTouchQueueCapacity
	}
	if config.TouchTimeout == 0 {
		config.TouchTimeout = defaultTouchTimeout
	}
	if config.TouchMinimumInterval == 0 {
		config.TouchMinimumInterval = defaultTouchMinimumInterval
	}
	if config.TouchGateEntries == 0 {
		config.TouchGateEntries = defaultTouchGateEntries
	}
	return config
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var (
	_ authentication.Verifier    = (*SessionAuthenticator)(nil)
	_ ports.SessionAuthenticator = (*SessionAuthenticator)(nil)
	_ lifecycle.Component        = (*SessionAuthenticator)(nil)
)
