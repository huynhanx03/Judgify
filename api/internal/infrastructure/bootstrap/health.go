package bootstrap

import (
	"context"
	"errors"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	internalent "github.com/huynhanx03/judgify/internal/ent"
)

var ErrDatabaseHealth = errors.New("bootstrap: database health unavailable")

type databaseHealthChecker struct {
	client *internalent.EntClient
}

// DatabaseHealthChecker returns a snapshot of the process-owned database
// client. It performs a bounded lightweight SQL probe when invoked by the
// shared health registry and never exposes its driver configuration.
func (process *Process) DatabaseHealthChecker() commonhealth.Checker {
	if process == nil {
		return databaseHealthChecker{}
	}
	process.mu.Lock()
	client := process.database
	process.mu.Unlock()
	return databaseHealthChecker{client: client}
}

func (checker databaseHealthChecker) Check(ctx context.Context) error {
	if ctx == nil || checker.client == nil || checker.client.Client() == nil {
		return ErrDatabaseHealth
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	rows, err := checker.client.Client().QueryContext(ctx, "SELECT 1")
	if err != nil {
		return ErrDatabaseHealth
	}
	if err := rows.Close(); err != nil {
		return ErrDatabaseHealth
	}
	return nil
}

var _ commonhealth.Checker = databaseHealthChecker{}
