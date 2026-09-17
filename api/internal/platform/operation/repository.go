package operation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository owns only durable control-plane state. Create, LockClaim, and
// CommitBatch require the caller's transaction scope when their documentation
// says so; domain work and CommitBatch therefore share one atomic commit.
type Repository interface {
	LockStart(context.Context, CreateCommand) (Operation, bool, error)
	Create(context.Context, CreateCommand) (CreateResult, error)
	Get(context.Context, uuid.UUID) (Operation, error)
	List(context.Context, ListFilter) (Page, error)

	Claim(context.Context, ClaimCommand) (Operation, bool, error)
	LockClaim(context.Context, Lease) (Operation, error)
	SettleRequestedControl(context.Context, Lease) (Operation, bool, error)
	CommitBatch(context.Context, Lease, BatchResult, time.Time) (Operation, error)
	ReleaseRetry(context.Context, Lease, RetryDirective) error
	MarkFailed(context.Context, Lease, string) error

	LockControl(context.Context, ControlCommand) (ControlResult, bool, error)
	ApplyControl(context.Context, ControlCommand, uuid.UUID) (Operation, error)
}

// UnitOfWork is intentionally narrow so operation does not expose Ent or
// acquire ownership of a domain repository.
type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
}

// Executor is process-local behavior registered under one closed operation
// kind. Only its canonical request/checkpoint data are durable.
type Executor interface {
	Kind() string
	ExecuteBatch(context.Context, BatchInput) (BatchResult, error)
}
