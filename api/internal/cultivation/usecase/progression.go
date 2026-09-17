package usecase

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/logger"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	platformmutation "github.com/huynhanx03/judgify/internal/platform/mutation"
	"github.com/huynhanx03/judgify/internal/ports"
)

const defaultMaximumManualProgressionDelta = int64(1_000_000)

type ProgressionServiceOptions struct {
	MaximumAbsoluteDelta int64
}

type progressionService struct {
	repository           ports.ProgressionRepository
	journal              platformmutation.Executor
	maximumAbsoluteDelta int64
}

func NewProgressionService(
	repository ports.ProgressionRepository,
	journal platformmutation.Executor,
	options ProgressionServiceOptions,
) (ports.ProgressionService, error) {
	if options.MaximumAbsoluteDelta == 0 {
		options.MaximumAbsoluteDelta = defaultMaximumManualProgressionDelta
	}
	if repository == nil || journal == nil || options.MaximumAbsoluteDelta < 1 ||
		options.MaximumAbsoluteDelta > entity.MaximumPublicProgressionBalance {
		return nil, entity.ErrInvalidProgressionCommand
	}
	return &progressionService{
		repository:           repository,
		journal:              journal,
		maximumAbsoluteDelta: options.MaximumAbsoluteDelta,
	}, nil
}

func (service *progressionService) PreviewAdjustment(
	ctx context.Context,
	userID uuid.UUID,
	delta int64,
	reversedEntryID *uuid.UUID,
) (entity.AdjustmentPreview, error) {
	if service == nil || ctx == nil || userID == uuid.Nil ||
		(reversedEntryID == nil && delta == 0) ||
		(reversedEntryID != nil && delta != 0) {
		return entity.AdjustmentPreview{}, badProgressionRequest(entity.ErrInvalidProgressionCommand)
	}
	if reversedEntryID != nil {
		var err error
		delta, err = service.repository.AdjustmentDelta(ctx, userID, *reversedEntryID)
		if err != nil {
			return entity.AdjustmentPreview{}, mapProgressionError(err)
		}
	}
	snapshot, err := service.repository.Snapshot(ctx, userID)
	if err != nil {
		return entity.AdjustmentPreview{}, mapProgressionError(err)
	}
	preview, err := entity.NewAdjustmentPreview(
		snapshot,
		delta,
		reversedEntryID,
		service.maximumAbsoluteDelta,
	)
	if err != nil {
		return entity.AdjustmentPreview{}, badProgressionRequest(err)
	}
	logger.FromContext(ctx).Debug(
		"progression adjustment previewed",
		zap.String("target_user_id", userID.String()),
		zap.Int64("expected_version", preview.ExpectedVersion),
		zap.Int64("delta", preview.Delta),
		zap.Bool("is_reversal", reversedEntryID != nil),
	)
	return preview, nil
}

func (service *progressionService) ApplyAdjustment(
	ctx context.Context,
	command entity.AdjustmentCommand,
) (entity.AdjustmentResult, error) {
	if service == nil || ctx == nil {
		return entity.AdjustmentResult{}, badProgressionRequest(entity.ErrInvalidProgressionCommand)
	}
	command, err := entity.NormalizeAdjustmentCommand(command, service.maximumAbsoluteDelta)
	if err != nil {
		return entity.AdjustmentResult{}, badProgressionRequest(err)
	}
	var result entity.AdjustmentResult
	err = service.journal.Execute(ctx, func(txCtx context.Context) (platformmutation.Command, error) {
		var applyErr error
		result, applyErr = service.repository.ApplyAdjustment(txCtx, command)
		if applyErr != nil {
			return platformmutation.Command{}, applyErr
		}
		if result.Replayed {
			return platformmutation.Command{Noop: true}, nil
		}
		action := "cultivation.progression.adjusted.v1"
		if command.ReversedEntryID != nil {
			action = "cultivation.progression.manual_reversed.v1"
		}
		return platformmutation.Command{
			EventID:    command.ID,
			Action:     action,
			Resource:   "cultivation_progression",
			ResourceID: command.UserID.String(),
			Reason:     command.Reason,
			Before: map[string]any{
				"balance": result.BalanceBefore,
				"version": result.Version - 1,
			},
			After: map[string]any{
				"balance":         result.BalanceAfter,
				"version":         result.Version,
				"ledger_entry_id": result.LedgerEntryID.String(),
				"incident_id":     command.IncidentID,
			},
			EventType:        action,
			AggregateType:    "cultivation_progression",
			AggregateID:      command.UserID,
			AggregateVersion: result.Version,
			Payload: map[string]any{
				"ledger_entry_id":     result.LedgerEntryID.String(),
				"user_id":             command.UserID.String(),
				"delta":               result.Delta,
				"balance_after":       result.BalanceAfter,
				"progression_version": result.Version,
			},
		}, nil
	})
	if err != nil {
		return entity.AdjustmentResult{}, mapProgressionError(err)
	}
	logger.FromContext(ctx).Info(
		"progression adjustment completed",
		zap.String("target_user_id", result.UserID.String()),
		zap.String("ledger_entry_id", result.LedgerEntryID.String()),
		zap.String("progression_version", strconv.FormatInt(result.Version, 10)),
		zap.Bool("idempotency_replayed", result.Replayed),
	)
	return result, nil
}

func (service *progressionService) ListLedger(
	ctx context.Context,
	userID uuid.UUID,
	cursor *uuid.UUID,
	limit int,
) (ports.ProgressionLedgerPage, error) {
	if service == nil || ctx == nil || userID == uuid.Nil {
		return ports.ProgressionLedgerPage{}, badProgressionRequest(entity.ErrInvalidProgressionCommand)
	}
	if limit == 0 {
		limit = 25
	}
	page, err := service.repository.ListLedger(ctx, userID, cursor, limit)
	if err != nil {
		return ports.ProgressionLedgerPage{}, mapProgressionError(err)
	}
	return page, nil
}

func mapProgressionError(err error) error {
	switch {
	case errors.Is(err, entity.ErrInvalidProgressionCommand):
		return badProgressionRequest(err)
	case errors.Is(err, entity.ErrProgressionConflict),
		errors.Is(err, entity.ErrProgressionAlreadyReversed):
		return apperr.New(apperr.CodeConflict, "progression state changed; preview again", err)
	case errors.Is(err, entity.ErrProgressionNotFound):
		return apperr.New(apperr.CodeNotFound, "progression record not found", err)
	default:
		return err
	}
}

func badProgressionRequest(err error) error {
	return apperr.New(apperr.CodeBadRequest, "invalid progression command", err)
}
