// Package eventing owns Judgify's transaction-bound integration-event outbox.
// Delivery and transport mapping are separate adapters; this package performs
// no network or queue I/O.
package eventing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	"github.com/huynhanx03/judgify/internal/platform/internal/boundedjson"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

const (
	maxEventTypeBytes     = 128
	maxAggregateTypeBytes = 64
	maxEventsPerAppend    = 100
	maxEventPayloadBytes  = 256 << 10
	maxEventBatchBytes    = 1 << 20
	maxEventPayloadDepth  = 32
	maxEventPayloadNodes  = 16 << 10
)

var (
	ErrInvalidEvent      = errors.New("eventing: invalid event")
	ErrDuplicateEvent    = errors.New("eventing: duplicate event identity")
	ErrPayloadTooLarge   = errors.New("eventing: payload too large")
	ErrPayloadTooDeep    = errors.New("eventing: payload too deep")
	ErrPayloadTooComplex = errors.New("eventing: payload too complex")

	eventTypePattern     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}(\.[a-z][a-z0-9_]{0,31}){1,5}\.v[1-9][0-9]{0,8}$`)
	aggregateTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// OutboxAppender atomically appends integration events to the UnitOfWork bound
// to ctx. It never publishes or owns a transaction lifecycle.
type OutboxAppender interface {
	Append(ctx context.Context, events ...Event) error
}

// Outbox is the Ent-backed transaction-bound appender.
type Outbox struct{}

// NewOutbox constructs a stateless appender.
func NewOutbox() *Outbox { return &Outbox{} }

var _ OutboxAppender = (*Outbox)(nil)

// SQLExecutor is the smallest raw transaction surface needed by AppendSQL.
// It exists for database owners that already hold a *sql.Tx and must include
// an outbox append in that exact transaction rather than opening a nested
// UnitOfWork.
type SQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type preparedEvent struct {
	id               uuid.UUID
	eventType        string
	aggregateType    string
	aggregateID      uuid.UUID
	aggregateVersion int64
	correlationID    string
	causationID      *uuid.UUID
	payload          json.RawMessage
	occurredAt       time.Time
}

type aggregateEventIdentity struct {
	eventType        string
	aggregateType    string
	aggregateID      uuid.UUID
	aggregateVersion int64
}

// Append validates and canonicalizes the whole bounded batch before issuing a
// single bulk insert through the transaction-scoped client.
func (*Outbox) Append(ctx context.Context, events ...Event) error {
	client, err := platformtx.Client(ctx, nil)
	if err != nil {
		return err
	}
	prepared, err := prepareEvents(ctx, events)
	if err != nil {
		return err
	}
	createdAt := time.Now().UTC()
	builders := make([]*generate.OutboxEventCreate, len(prepared))
	for index, event := range prepared {
		builders[index] = client.OutboxEvent.Create().
			SetID(event.id).
			SetEventType(event.eventType).
			SetAggregateType(event.aggregateType).
			SetAggregateID(event.aggregateID).
			SetAggregateVersion(event.aggregateVersion).
			SetCorrelationID(event.correlationID).
			SetNillableCausationID(event.causationID).
			SetPayload(event.payload).
			SetOccurredAt(event.occurredAt).
			SetCreatedAt(createdAt).
			SetNextAttemptAt(createdAt)
	}
	if _, err := client.OutboxEvent.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("append outbox events: %w", err)
	}
	return nil
}

// AppendSQL applies the same validation, canonicalization, batching, and
// duplicate-in-batch checks as Outbox.Append, but writes through a caller-owned
// raw SQL transaction. The executor is deliberately incapable of committing.
func AppendSQL(
	ctx context.Context,
	executor SQLExecutor,
	events ...Event,
) error {
	if executor == nil {
		return ErrInvalidEvent
	}
	prepared, err := prepareEvents(ctx, events)
	if err != nil {
		return err
	}
	createdAt := time.Now().UTC()
	const columnsPerEvent = 11
	arguments := make([]any, 0, len(prepared)*columnsPerEvent)
	var statement strings.Builder
	statement.WriteString(`
		INSERT INTO outbox_events (
			id, event_type, aggregate_type, aggregate_id, aggregate_version,
			correlation_id, causation_id, payload, occurred_at, created_at,
			next_attempt_at
		) VALUES `)
	for index, event := range prepared {
		if index != 0 {
			statement.WriteByte(',')
		}
		statement.WriteByte('(')
		for column := 0; column < columnsPerEvent; column++ {
			if column != 0 {
				statement.WriteByte(',')
			}
			statement.WriteByte('$')
			statement.WriteString(strconv.Itoa(index*columnsPerEvent + column + 1))
		}
		statement.WriteByte(')')
		arguments = append(
			arguments,
			event.id,
			event.eventType,
			event.aggregateType,
			event.aggregateID,
			event.aggregateVersion,
			event.correlationID,
			event.causationID,
			event.payload,
			event.occurredAt,
			createdAt,
			createdAt,
		)
	}
	if _, err := executor.ExecContext(ctx, statement.String(), arguments...); err != nil {
		return fmt.Errorf("append outbox events: %w", err)
	}
	return nil
}

func prepareEvents(ctx context.Context, events []Event) ([]preparedEvent, error) {
	if len(events) == 0 || len(events) > maxEventsPerAppend {
		return nil, ErrInvalidEvent
	}
	prepared := make([]preparedEvent, len(events))
	identities := make(map[aggregateEventIdentity]struct{}, len(events))
	eventIDs := make(map[uuid.UUID]struct{}, len(events))
	totalPayloadBytes := 0
	limits := boundedjson.Limits{
		MaxBytes: maxEventPayloadBytes,
		MaxDepth: maxEventPayloadDepth,
		MaxNodes: maxEventPayloadNodes,
	}
	for index, event := range events {
		if event.ID == uuid.Nil ||
			len(event.Type) > maxEventTypeBytes || !eventTypePattern.MatchString(event.Type) ||
			len(event.AggregateType) > maxAggregateTypeBytes || !aggregateTypePattern.MatchString(event.AggregateType) ||
			event.AggregateID == uuid.Nil || event.AggregateVersion <= 0 ||
			event.OccurredAt.IsZero() ||
			(event.CausationID != nil && *event.CausationID == uuid.Nil) {
			return nil, ErrInvalidEvent
		}
		correlationID := event.CorrelationID
		if correlationID == "" {
			correlationID = correlation.FromContext(ctx)
		}
		if correlation.Validate(correlationID) != nil {
			return nil, ErrInvalidEvent
		}
		if _, duplicate := eventIDs[event.ID]; duplicate {
			return nil, ErrDuplicateEvent
		}
		eventIDs[event.ID] = struct{}{}
		identity := aggregateEventIdentity{
			eventType:        event.Type,
			aggregateType:    event.AggregateType,
			aggregateID:      event.AggregateID,
			aggregateVersion: event.AggregateVersion,
		}
		if _, duplicate := identities[identity]; duplicate {
			return nil, ErrDuplicateEvent
		}
		identities[identity] = struct{}{}

		payload, payloadErr := boundedjson.CanonicalObject(event.Payload, limits)
		if payloadErr != nil {
			return nil, mapPayloadError(payloadErr)
		}
		if len(payload) > maxEventBatchBytes-totalPayloadBytes {
			return nil, ErrPayloadTooLarge
		}
		totalPayloadBytes += len(payload)
		prepared[index] = preparedEvent{
			id:               event.ID,
			eventType:        strings.Clone(event.Type),
			aggregateType:    strings.Clone(event.AggregateType),
			aggregateID:      event.AggregateID,
			aggregateVersion: event.AggregateVersion,
			correlationID:    strings.Clone(correlationID),
			causationID:      cloneEventUUID(event.CausationID),
			payload:          append(json.RawMessage(nil), payload...),
			occurredAt:       event.OccurredAt.UTC(),
		}
	}
	return prepared, nil
}

func mapPayloadError(err error) error {
	switch {
	case errors.Is(err, boundedjson.ErrTooLarge):
		return ErrPayloadTooLarge
	case errors.Is(err, boundedjson.ErrTooDeep):
		return ErrPayloadTooDeep
	case errors.Is(err, boundedjson.ErrTooComplex):
		return ErrPayloadTooComplex
	default:
		return ErrInvalidEvent
	}
}

func cloneEventUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
