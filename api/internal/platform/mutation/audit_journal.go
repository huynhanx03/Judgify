package mutation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/platform/audit"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

// AuditJournal atomically records an aggregate write and its immutable audit
// evidence when the aggregate has no durable version suitable for outbox
// ordering. It deliberately cannot emit integration events.
type AuditJournal struct {
	transactions platformtx.UnitOfWork
	audit        audit.Appender
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

func NewAuditJournal(transactions platformtx.UnitOfWork, auditAppender audit.Appender) *AuditJournal {
	return &AuditJournal{
		transactions: transactions,
		audit:        auditAppender,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}
}

func (journal *AuditJournal) Execute(
	ctx context.Context,
	mutate func(context.Context) (Command, error),
) error {
	if journal == nil || journal.transactions == nil || journal.audit == nil ||
		journal.now == nil || journal.newID == nil || ctx == nil || mutate == nil {
		return platformtx.ErrInvalidUnitOfWork
	}
	ctx = correlation.EnsureContext(ctx)
	return journal.transactions.Within(ctx, func(txCtx context.Context) error {
		command, err := mutate(txCtx)
		if err != nil {
			return err
		}
		if command.Noop {
			return nil
		}
		id, err := journal.newID()
		if err != nil {
			return fmt.Errorf("create mutation audit identity: %w", err)
		}
		actorUserID, actorSessionID := mutationActor(txCtx)
		_, err = journal.audit.Append(txCtx, audit.Entry{
			ID: id, ActorUserID: actorUserID, ActorSessionID: actorSessionID,
			Action: command.Action, Resource: command.Resource, ResourceID: command.ResourceID,
			Reason: command.Reason, Before: command.Before, After: command.After,
			CorrelationID: correlation.FromContext(txCtx), OccurredAt: journal.now().UTC(),
		})
		if err != nil {
			return fmt.Errorf("append mutation audit: %w", err)
		}
		return nil
	})
}

var _ Executor = (*AuditJournal)(nil)
