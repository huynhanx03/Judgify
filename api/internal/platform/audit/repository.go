package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	"github.com/huynhanx03/judgify/internal/platform/internal/boundedjson"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

const (
	maxAuditActionBytes   = 128
	maxAuditResourceBytes = 128
	maxAuditResourceID    = 256
	maxAuditReasonBytes   = 1024

	maxAuditDocumentBytes = 32 << 10
	maxAuditTotalBytes    = 64 << 10
	maxAuditDepth         = 8
	maxAuditNodes         = 512
)

var (
	ErrInvalidEntry    = errors.New("audit: invalid entry")
	ErrEntryTooLarge   = errors.New("audit: entry too large")
	ErrEntryTooDeep    = errors.New("audit: entry too deep")
	ErrEntryTooComplex = errors.New("audit: entry too complex")
)

var defaultRedactedKeys = []string{
	"password",
	"password_hash",
	"token",
	"access_token",
	"refresh_token",
	"reset_token",
	"verification_token",
	"secret",
	"client_secret",
	"oauth_secret",
	"private_key",
	"source_code",
	"hidden_testcase",
	"actor_session_id",
	"session_id",
}

// Option configures audit preparation without weakening hard safety limits.
type Option func(*Repository)

// WithRedactedKeys adds case- and separator-insensitive object keys to the
// mandatory redaction set.
func WithRedactedKeys(keys ...string) Option {
	cloned := append([]string(nil), keys...)
	return func(repository *Repository) {
		for _, key := range cloned {
			if normalized := normalizeKey(key); normalized != "" {
				repository.redacted[normalized] = struct{}{}
			}
		}
	}
}

// Repository persists append-only audit rows through a transaction scope.
type Repository struct {
	redacted map[string]struct{}
	now      func() time.Time
}

// NewRepository constructs a safe repository with mandatory secret redaction.
func NewRepository(options ...Option) *Repository {
	repository := &Repository{
		redacted: make(map[string]struct{}, len(defaultRedactedKeys)),
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, key := range defaultRedactedKeys {
		repository.redacted[normalizeKey(key)] = struct{}{}
	}
	for _, option := range options {
		if option != nil {
			option(repository)
		}
	}
	return repository
}

var _ Appender = (*Repository)(nil)

// SQLExecutor is the non-owning raw transaction surface used by AppendSQL.
// Implementations may execute statements but cannot commit or roll back.
type SQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type preparedEntry struct {
	id             uuid.UUID
	actorUserID    *uuid.UUID
	actorSessionID *uuid.UUID
	action         string
	resource       string
	resourceID     string
	reason         string
	beforeJSON     json.RawMessage
	afterJSON      json.RawMessage
	correlationID  string
	occurredAt     time.Time
	createdAt      time.Time
}

// Append requires an active UnitOfWork and adds exactly one immutable row to
// that transaction. It deliberately exposes no update or delete operation.
func (repository *Repository) Append(ctx context.Context, entry Entry) (uuid.UUID, error) {
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	prepared, err := repository.prepareEntry(ctx, entry)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = client.AuditLog.Create().
		SetID(prepared.id).
		SetNillableActorUserID(prepared.actorUserID).
		SetNillableActorSessionID(prepared.actorSessionID).
		SetAction(prepared.action).
		SetResource(prepared.resource).
		SetResourceID(prepared.resourceID).
		SetReason(prepared.reason).
		SetBeforeData(prepared.beforeJSON).
		SetAfterData(prepared.afterJSON).
		SetCorrelationID(prepared.correlationID).
		SetOccurredAt(prepared.occurredAt).
		SetCreatedAt(prepared.createdAt).
		Save(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("append audit entry: %w", err)
	}
	return prepared.id, nil
}

// AppendSQL prepares and redacts an entry through the same policy as Append,
// then writes it through an already-owned raw transaction. This is intended
// for domain repositories whose existing transaction cannot be represented by
// a nested Ent UnitOfWork.
func (repository *Repository) AppendSQL(
	ctx context.Context,
	executor SQLExecutor,
	entry Entry,
) (uuid.UUID, error) {
	if executor == nil {
		return uuid.Nil, ErrInvalidEntry
	}
	prepared, err := repository.prepareEntry(ctx, entry)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := executor.ExecContext(ctx, `
		INSERT INTO audit_logs (
			id, actor_user_id, actor_session_id, action, resource, resource_id,
			reason, before_data, after_data, correlation_id, occurred_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)`,
		prepared.id,
		prepared.actorUserID,
		prepared.actorSessionID,
		prepared.action,
		prepared.resource,
		prepared.resourceID,
		prepared.reason,
		prepared.beforeJSON,
		prepared.afterJSON,
		prepared.correlationID,
		prepared.occurredAt,
		prepared.createdAt,
	); err != nil {
		return uuid.Nil, fmt.Errorf("append audit entry: %w", err)
	}
	return prepared.id, nil
}

func (repository *Repository) prepareEntry(ctx context.Context, entry Entry) (preparedEntry, error) {
	if repository == nil || repository.now == nil || repository.redacted == nil {
		return preparedEntry{}, ErrInvalidEntry
	}
	if !validIdentifier(entry.Action, maxAuditActionBytes) ||
		!validIdentifier(entry.Resource, maxAuditResourceBytes) ||
		!validText(entry.ResourceID, maxAuditResourceID, true) ||
		!validText(entry.Reason, maxAuditReasonBytes, false) {
		return preparedEntry{}, ErrInvalidEntry
	}
	if entry.ActorUserID != nil && *entry.ActorUserID == uuid.Nil {
		return preparedEntry{}, ErrInvalidEntry
	}
	if entry.ActorSessionID != nil && *entry.ActorSessionID == uuid.Nil {
		return preparedEntry{}, ErrInvalidEntry
	}

	correlationID := entry.CorrelationID
	if correlationID == "" {
		correlationID = correlation.FromContext(ctx)
	}
	if correlation.Validate(correlationID) != nil {
		return preparedEntry{}, ErrInvalidEntry
	}

	limits := boundedjson.Limits{
		MaxBytes: maxAuditDocumentBytes,
		MaxDepth: maxAuditDepth,
		MaxNodes: maxAuditNodes,
	}
	redact := func(key string) bool {
		_, sensitive := repository.redacted[normalizeKey(key)]
		return sensitive
	}
	before, err := boundedjson.MarshalMap(entry.Before, limits, redact)
	if err != nil {
		return preparedEntry{}, mapJSONError(err)
	}
	after, err := boundedjson.MarshalMap(entry.After, limits, redact)
	if err != nil {
		return preparedEntry{}, mapJSONError(err)
	}
	if len(before) > maxAuditTotalBytes-len(after) {
		return preparedEntry{}, ErrEntryTooLarge
	}

	id := entry.ID
	if id == uuid.Nil {
		id = commonent.NewUUID()
	}
	now := repository.now().UTC()
	occurredAt := entry.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	} else {
		occurredAt = occurredAt.UTC()
	}
	return preparedEntry{
		id:             id,
		actorUserID:    cloneUUID(entry.ActorUserID),
		actorSessionID: cloneUUID(entry.ActorSessionID),
		action:         strings.Clone(entry.Action),
		resource:       strings.Clone(entry.Resource),
		resourceID:     strings.Clone(entry.ResourceID),
		reason:         strings.Clone(entry.Reason),
		beforeJSON:     append(json.RawMessage(nil), before...),
		afterJSON:      append(json.RawMessage(nil), after...),
		correlationID:  strings.Clone(correlationID),
		occurredAt:     occurredAt,
		createdAt:      now,
	}, nil
}

func mapJSONError(err error) error {
	switch {
	case errors.Is(err, boundedjson.ErrTooLarge):
		return ErrEntryTooLarge
	case errors.Is(err, boundedjson.ErrTooDeep):
		return ErrEntryTooDeep
	case errors.Is(err, boundedjson.ErrTooComplex):
		return ErrEntryTooComplex
	default:
		return ErrInvalidEntry
	}
}

func validIdentifier(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '.', '_', ':', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func validText(value string, maximum int, emptyAllowed bool) bool {
	if len(value) > maximum || !utf8.ValidString(value) || value != strings.TrimSpace(value) {
		return false
	}
	if value == "" {
		return emptyAllowed
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func normalizeKey(value string) string {
	var normalized strings.Builder
	normalized.Grow(len(value))
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			normalized.WriteRune(character)
		}
	}
	return normalized.String()
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
