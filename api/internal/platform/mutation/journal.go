// Package mutation coordinates one durable domain mutation with its audit and
// integration-event records. It intentionally owns no persistence details of
// the aggregate itself; callers provide that bounded work as a callback.
package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

// Journal is the reusable transaction-bound command envelope for ordinary
// administrative mutations. Aggregate writes, immutable audit evidence, and
// the durable outbox event either commit together or are all rolled back.
type Journal struct {
	transactions platformtx.UnitOfWork
	audit        audit.Appender
	outbox       eventing.OutboxAppender
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

// Executor is the common transaction envelope implemented by versioned
// audit+outbox journals and audit-only journals for unversioned aggregates.
type Executor interface {
	Execute(context.Context, func(context.Context) (Command, error)) error
}

// Command describes the immutable evidence to append after a mutation has
// succeeded. Before and After must be bounded JSON-shaped documents.
type Command struct {
	// Noop marks an idempotent replay that must commit no new audit/outbox
	// evidence. The original command already owns that immutable evidence.
	Noop             bool
	EventID          uuid.UUID
	Action           string
	Resource         string
	ResourceID       string
	Reason           string
	Before           map[string]any
	After            map[string]any
	EventType        string
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	Payload          map[string]any
}

// NewJournal creates a production command journal. Nil dependencies are kept
// invalid rather than panicking so a configuration error fails at execution.
func NewJournal(
	transactions platformtx.UnitOfWork,
	auditAppender audit.Appender,
	outboxAppender eventing.OutboxAppender,
) *Journal {
	return &Journal{
		transactions: transactions,
		audit:        auditAppender,
		outbox:       outboxAppender,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}
}

// Execute runs mutate and journals the command it returns in one UnitOfWork.
// Building the command after the write lets database-assigned identities and
// timestamps participate in the durable evidence. The returned mutation error
// is preserved unchanged so existing error contracts remain at the service
// boundary.
func (journal *Journal) Execute(
	ctx context.Context,
	mutate func(context.Context) (Command, error),
) error {
	if journal == nil || journal.transactions == nil || journal.audit == nil || journal.outbox == nil ||
		journal.now == nil || journal.newID == nil || ctx == nil || mutate == nil {
		return platformtx.ErrInvalidUnitOfWork
	}
	ctx = correlation.EnsureContext(ctx)
	return journal.transactions.Within(ctx, func(txCtx context.Context) error {
		command, mutationErr := mutate(txCtx)
		if mutationErr != nil {
			return mutationErr
		}
		return journal.append(txCtx, command)
	})
}

func (journal *Journal) append(ctx context.Context, command Command) error {
	if command.Noop {
		return nil
	}
	eventID := command.EventID
	if eventID == uuid.Nil {
		var err error
		eventID, err = journal.newID()
		if err != nil {
			return fmt.Errorf("create mutation event identity: %w", err)
		}
	}
	occurredAt := journal.now().UTC()
	if command.AggregateVersion <= 0 {
		return eventing.ErrInvalidEvent
	}
	payload, err := json.Marshal(command.Payload)
	if err != nil {
		return fmt.Errorf("encode mutation event: %w", err)
	}
	actorUserID, actorSessionID := mutationActor(ctx)
	correlationID := correlation.FromContext(ctx)
	if _, err := journal.audit.Append(ctx, audit.Entry{
		ID: eventID, ActorUserID: actorUserID, ActorSessionID: actorSessionID,
		Action: command.Action, Resource: command.Resource, ResourceID: command.ResourceID,
		Reason: command.Reason, Before: command.Before, After: command.After,
		CorrelationID: correlationID, OccurredAt: occurredAt,
	}); err != nil {
		return fmt.Errorf("append mutation audit: %w", err)
	}
	if err := journal.outbox.Append(ctx, eventing.Event{
		ID: eventID, Type: command.EventType, AggregateType: command.AggregateType,
		AggregateID: command.AggregateID, AggregateVersion: command.AggregateVersion,
		CorrelationID: correlationID, Payload: payload, OccurredAt: occurredAt,
	}); err != nil {
		return fmt.Errorf("append mutation event: %w", err)
	}
	return nil
}

var _ Executor = (*Journal)(nil)

func mutationActor(ctx context.Context) (*uuid.UUID, *uuid.UUID) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	var user, session *uuid.UUID
	if userOK && userID != uuid.Nil {
		value := userID
		user = &value
	}
	if sessionOK && sessionID != uuid.Nil {
		value := sessionID
		session = &value
	}
	return user, session
}
