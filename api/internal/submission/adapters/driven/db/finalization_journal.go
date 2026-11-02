package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
)

var ErrInvalidFinalizationJournal = errors.New(
	"submission finalization journal: invalid operation",
)

type finalizationSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// finalizationJournal is intentionally generic over validated event/audit
// records. Submission finalization and expired-lease reconciliation can share
// one raw transaction path without teaching the platform journal judge logic.
type finalizationJournal interface {
	Append(
		context.Context,
		finalizationSQLExecutor,
		[]eventing.Event,
		audit.Entry,
	) error
}

type durableFinalizationJournal struct {
	audit *audit.Repository
}

func newDurableFinalizationJournal() *durableFinalizationJournal {
	return &durableFinalizationJournal{audit: audit.NewRepository()}
}

func (journal *durableFinalizationJournal) Append(
	ctx context.Context,
	executor finalizationSQLExecutor,
	events []eventing.Event,
	entry audit.Entry,
) error {
	if journal == nil || journal.audit == nil || ctx == nil ||
		executor == nil || len(events) == 0 {
		return ErrInvalidFinalizationJournal
	}
	if err := eventing.AppendSQL(ctx, executor, events...); err != nil {
		return fmt.Errorf("append finalization outbox: %w", err)
	}
	if _, err := journal.audit.AppendSQL(ctx, executor, entry); err != nil {
		return fmt.Errorf("append finalization audit: %w", err)
	}
	return nil
}

var _ finalizationJournal = (*durableFinalizationJournal)(nil)
