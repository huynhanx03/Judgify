package operation

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"

	"github.com/huynhanx03/judgify/internal/platform/audit"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

var ErrInvalidStarter = errors.New("operation: invalid starter")

type StartRequest struct {
	Operation CreateCommand
	Audit     audit.Entry
}

// Starter serializes the exact idempotency tuple before appending audit, then
// persists both records in one caller-independent transaction. Concurrent and
// lost-response retries therefore return the first committed operation without
// producing a duplicate audit row.
type Starter struct {
	repository         Repository
	transactions       UnitOfWork
	audit              audit.Appender
	requireTransaction func(context.Context) error
}

func NewStarter(
	repository Repository,
	transactions UnitOfWork,
	auditAppender audit.Appender,
) (*Starter, error) {
	if repository == nil || transactions == nil || auditAppender == nil {
		return nil, ErrInvalidStarter
	}
	return &Starter{
		repository: repository, transactions: transactions, audit: auditAppender,
		requireTransaction: requireActiveTransaction,
	}, nil
}

func (starter *Starter) Start(
	ctx context.Context,
	request StartRequest,
) (CreateResult, error) {
	ctx, request, err := starter.prepare(ctx, request)
	if err != nil {
		return CreateResult{}, err
	}
	var result CreateResult
	err = starter.transactions.Within(ctx, func(txCtx context.Context) error {
		var startErr error
		result, startErr = starter.startPrepared(txCtx, request)
		return startErr
	})
	if err != nil {
		return CreateResult{}, err
	}
	return result, nil
}

// StartInTransaction starts or exactly replays an operation inside the
// caller's already-active UnitOfWork. It never opens or nests a transaction,
// so a domain can freeze its aggregate, append recipients, audit, and create
// the operation as one atomic commit.
func (starter *Starter) StartInTransaction(
	ctx context.Context,
	request StartRequest,
) (CreateResult, error) {
	if starter == nil || starter.requireTransaction == nil {
		return CreateResult{}, ErrInvalidStarter
	}
	if err := starter.requireTransaction(ctx); err != nil {
		return CreateResult{}, err
	}
	ctx, request, err := starter.prepare(ctx, request)
	if err != nil {
		return CreateResult{}, err
	}
	return starter.startPrepared(ctx, request)
}

func (starter *Starter) prepare(
	ctx context.Context,
	request StartRequest,
) (context.Context, StartRequest, error) {
	if starter == nil || starter.repository == nil || starter.transactions == nil ||
		starter.audit == nil || starter.requireTransaction == nil ||
		ctx == nil || ctx.Err() != nil ||
		request.Audit.Action == "" || request.Audit.Resource == "" {
		return nil, StartRequest{}, ErrInvalidStarter
	}
	ctx = correlation.EnsureContext(ctx)
	if request.Operation.ID == uuid.Nil {
		generated, err := uuid.NewV7()
		if err != nil {
			return nil, StartRequest{}, ErrInvalidStarter
		}
		request.Operation.ID = generated
	}
	if request.Operation.CorrelationID == "" {
		request.Operation.CorrelationID = correlation.FromContext(ctx)
	}
	if len(request.Operation.Checkpoint) == 0 {
		request.Operation.Checkpoint = EmptyCheckpoint()
	}
	if ValidateCreateIntent(request.Operation) != nil {
		return nil, StartRequest{}, ErrInvalidStarter
	}
	if request.Audit.ActorUserID != nil &&
		*request.Audit.ActorUserID != request.Operation.ActorUserID {
		return nil, StartRequest{}, ErrInvalidStarter
	}
	if request.Audit.ActorSessionID != nil &&
		(request.Operation.ActorSessionID == nil ||
			*request.Audit.ActorSessionID != *request.Operation.ActorSessionID) {
		return nil, StartRequest{}, ErrInvalidStarter
	}
	return ctx, request, nil
}

func (starter *Starter) startPrepared(
	ctx context.Context,
	request StartRequest,
) (CreateResult, error) {
	existing, found, err := starter.repository.LockStart(ctx, request.Operation)
	if err != nil {
		return CreateResult{}, err
	}
	if found {
		if !sameStartIntent(existing, request.Operation) {
			return CreateResult{}, ErrConflict
		}
		return CreateResult{Operation: existing, Replay: true}, nil
	}

	entry := request.Audit
	entry.ID = uuid.Nil
	actorUserID := request.Operation.ActorUserID
	entry.ActorUserID = &actorUserID
	entry.ActorSessionID = cloneUUID(request.Operation.ActorSessionID)
	entry.ResourceID = request.Operation.ID.String()
	entry.Reason = request.Operation.Reason
	entry.CorrelationID = request.Operation.CorrelationID
	auditID, err := starter.audit.Append(ctx, entry)
	if err != nil {
		return CreateResult{}, err
	}
	command := request.Operation
	command.AuditID = auditID
	created, err := starter.repository.Create(ctx, command)
	if err != nil {
		return CreateResult{}, err
	}
	if created.Replay {
		return CreateResult{}, ErrConflict
	}
	return created, nil
}

func requireActiveTransaction(ctx context.Context) error {
	_, err := platformtx.Client(ctx, nil)
	return err
}

func sameStartIntent(existing Operation, command CreateCommand) bool {
	if existing.Kind != command.Kind ||
		existing.IdempotencyKey != command.IdempotencyKey ||
		existing.RequestHash != command.RequestHash ||
		!bytes.Equal(existing.Request, command.Request) ||
		existing.Total != command.Total ||
		existing.BatchSize != command.BatchSize ||
		!bytes.Equal(existing.Checkpoint, command.Checkpoint) ||
		existing.ActorUserID != command.ActorUserID ||
		existing.Reason != command.Reason ||
		!sameOptionalTime(existing.NotBefore, command.NotBefore) {
		return false
	}
	switch {
	case existing.ActorSessionID == nil && command.ActorSessionID == nil:
		return true
	case existing.ActorSessionID == nil || command.ActorSessionID == nil:
		return false
	default:
		return *existing.ActorSessionID == *command.ActorSessionID
	}
}

func sameOptionalTime(left, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Equal(*right) && left.Location() == right.Location()
	}
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
