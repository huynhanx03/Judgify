package db

import (
	"context"
	stdsql "database/sql"
	"time"

	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

type rewardTimelineExecutor interface {
	ExecContext(context.Context, string, ...any) (stdsql.Result, error)
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
}

func acquireRewardTimelineLock(
	ctx context.Context,
	executor rewardTimelineExecutor,
) error {
	_, err := executor.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		platformtx.RewardTimelineAdvisoryLock,
	)
	return err
}

// rewardTimelineNow is deliberately called only after the candidate command
// has been locked and validated. Its value is therefore the linearization
// point used by effective-dated reward history.
func rewardTimelineNow(
	ctx context.Context,
	executor rewardTimelineExecutor,
) (time.Time, error) {
	var value time.Time
	err := querySingleRow(ctx, executor, `SELECT clock_timestamp()`).Scan(&value)
	return value.UTC(), err
}
