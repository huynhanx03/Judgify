package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/ports"
)

const maximumJudgeOperationReasonBytes = 1024

var ErrInvalidJudgeOperation = errors.New("submission operations: invalid command")

type judgeOperations struct {
	repository ports.SubmissionOperationsRepository
}

// NewJudgeOperations creates the narrow service boundary for administrative
// judge actions. It accepts neither a generic update model nor client-supplied
// execution configuration.
func NewJudgeOperations(
	repository ports.SubmissionOperationsRepository,
) (ports.SubmissionOperations, error) {
	if repository == nil {
		return nil, ports.ErrOperationUnavailable
	}
	return &judgeOperations{repository: repository}, nil
}

func (service *judgeOperations) Rejudge(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.RejudgeCommand,
) (ports.OperationResult, error) {
	if service == nil || service.repository == nil || ctx == nil ||
		!validOperationActor(actor) || !validRejudgeCommand(command) {
		return ports.OperationResult{}, ErrInvalidJudgeOperation
	}
	ctx = logger.WithFields(
		ctx,
		logger.String("submission_id", command.SubmissionID.String()),
		logger.String("operation", "rejudge"),
		logger.String("actor_user_id", actor.UserID.String()),
		logger.String("expected_submission_version", strconv.FormatInt(command.ExpectedVersion, 10)),
	)
	log := logger.FromContext(ctx)
	log.Debug("judge rejudge operation started")
	result, err := service.repository.Rejudge(ctx, actor, command)
	if err != nil {
		log.Warn("judge rejudge operation rejected", zap.Error(err))
		return ports.OperationResult{}, err
	}
	log.Info("judge rejudge operation committed",
		zap.Int64("generation", result.Generation),
		zap.Int64("submission_version", result.AggregateVersion),
		zap.Bool("idempotent_replay", result.IdempotentReplay),
	)
	return result, nil
}

func (service *judgeOperations) Cancel(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.CancelCommand,
) (ports.OperationResult, error) {
	if service == nil || service.repository == nil || ctx == nil ||
		!validOperationActor(actor) || !validCancelCommand(command) {
		return ports.OperationResult{}, ErrInvalidJudgeOperation
	}
	ctx = logger.WithFields(
		ctx,
		logger.String("submission_id", command.SubmissionID.String()),
		logger.String("operation", "cancel"),
		logger.String("actor_user_id", actor.UserID.String()),
		logger.String("target_generation", strconv.FormatInt(command.Generation, 10)),
		logger.String("expected_submission_version", strconv.FormatInt(command.ExpectedVersion, 10)),
	)
	log := logger.FromContext(ctx)
	log.Debug("judge cancellation operation started")
	result, err := service.repository.Cancel(ctx, actor, command)
	if err != nil {
		log.Warn("judge cancellation operation rejected", zap.Error(err))
		return ports.OperationResult{}, err
	}
	log.Info("judge cancellation operation committed",
		zap.Int64("submission_version", result.AggregateVersion),
		zap.String("phase", string(result.Phase)),
	)
	return result, nil
}

func validOperationActor(actor ports.OperationActor) bool {
	return actor.UserID != uuid.Nil &&
		(actor.SessionID == nil || *actor.SessionID != uuid.Nil)
}

func validRejudgeCommand(command ports.RejudgeCommand) bool {
	return command.SubmissionID != uuid.Nil && command.ExpectedVersion > 0 &&
		command.IdempotencyKey != uuid.Nil && validJudgeOperationReason(command.Reason)
}

func validCancelCommand(command ports.CancelCommand) bool {
	return command.SubmissionID != uuid.Nil && command.Generation > 0 &&
		command.ExpectedVersion > 0 && validJudgeOperationReason(command.Reason)
}

func validJudgeOperationReason(reason string) bool {
	if len(reason) == 0 || len(reason) > maximumJudgeOperationReasonBytes ||
		strings.TrimSpace(reason) == "" || !utf8.ValidString(reason) {
		return false
	}
	for _, character := range reason {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

var _ ports.SubmissionOperations = (*judgeOperations)(nil)
