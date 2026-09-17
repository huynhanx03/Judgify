package operation

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/audit"
)

var ErrInvalidController = errors.New("operation: invalid controller")

type Controller struct {
	repository   Repository
	transactions UnitOfWork
	audit        audit.Appender
}

func NewController(
	repository Repository,
	transactions UnitOfWork,
	auditAppender audit.Appender,
) (*Controller, error) {
	if repository == nil || transactions == nil || auditAppender == nil {
		return nil, ErrInvalidController
	}
	return &Controller{
		repository: repository, transactions: transactions, audit: auditAppender,
	}, nil
}

func (controller *Controller) Control(
	ctx context.Context,
	command ControlCommand,
) (ControlResult, error) {
	if err := controller.validate(ctx, command); err != nil {
		return ControlResult{}, err
	}
	var result ControlResult
	err := controller.transactions.Within(ctx, func(txCtx context.Context) error {
		var controlErr error
		result, controlErr = controller.controlPrepared(txCtx, command)
		return controlErr
	})
	if err != nil {
		return ControlResult{}, err
	}
	return result, nil
}

// ControlInTransaction applies an idempotent control command inside a
// caller-owned transaction. Domain workflows use it when their aggregate
// transition, the generic operation transition, and both audit records must
// commit atomically. It deliberately never opens or nests a transaction.
func (controller *Controller) ControlInTransaction(
	ctx context.Context,
	command ControlCommand,
) (ControlResult, error) {
	if err := controller.validate(ctx, command); err != nil {
		return ControlResult{}, err
	}
	if err := requireActiveTransaction(ctx); err != nil {
		return ControlResult{}, err
	}
	return controller.controlPrepared(ctx, command)
}

func (controller *Controller) validate(
	ctx context.Context,
	command ControlCommand,
) error {
	if controller == nil || controller.repository == nil ||
		controller.transactions == nil || controller.audit == nil ||
		ctx == nil || ctx.Err() != nil {
		return ErrInvalidController
	}
	if _, err := ControlRequestHash(command); err != nil {
		return err
	}
	return nil
}

func (controller *Controller) controlPrepared(
	ctx context.Context,
	command ControlCommand,
) (ControlResult, error) {
	existing, found, err := controller.repository.LockControl(ctx, command)
	if err != nil {
		return ControlResult{}, err
	}
	if found {
		existing.Replay = true
		return existing, nil
	}
	before, err := controller.repository.Get(ctx, command.OperationID)
	if err != nil {
		return ControlResult{}, err
	}
	actorUserID := command.ActorUserID
	actorSessionID := command.ActorSessionID
	auditID, err := uuid.NewV7()
	if err != nil {
		return ControlResult{}, ErrInvalidController
	}
	updated, err := controller.repository.ApplyControl(ctx, command, auditID)
	if err != nil {
		return ControlResult{}, err
	}
	persistedAuditID, err := controller.audit.Append(ctx, audit.Entry{
		ID:          auditID,
		ActorUserID: &actorUserID, ActorSessionID: &actorSessionID,
		Action:   "operation." + string(command.Action),
		Resource: "operation", ResourceID: command.OperationID.String(),
		Reason: command.Reason,
		Before: map[string]any{
			"status":  before.Status,
			"version": before.Version,
		},
		After: map[string]any{
			"status":           updated.Status,
			"version":          updated.Version,
			"requested_action": updated.RequestedAction,
		},
	})
	if err != nil {
		return ControlResult{}, err
	}
	if persistedAuditID != auditID {
		return ControlResult{}, ErrInvalidController
	}
	projected, err := Project(updated)
	if err != nil {
		return ControlResult{}, err
	}
	return ControlResult{Operation: projected, AuditID: auditID}, nil
}
