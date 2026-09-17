package usecase

import (
	"context"
	"errors"
	"reflect"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	intake "github.com/huynhanx03/judgify/internal/application/workflow/submissionintake"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

var ErrSubmissionCommandUnavailable = errors.New(
	"submission: durable command unavailable",
)

type intakeExecutor interface {
	Execute(context.Context, intake.Command) (intake.Result, error)
}

type submissionCommandService struct {
	executor intakeExecutor
}

func NewSubmissionCommandService(
	executor intakeExecutor,
) (ports.SubmissionCommandService, error) {
	if nilIntakeExecutor(executor) {
		return nil, ErrSubmissionCommandUnavailable
	}
	return &submissionCommandService{executor: executor}, nil
}

func (service *submissionCommandService) Submit(
	ctx context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	command *contract.CreateSubmissionCommand,
) (*contract.SubmissionAcceptedResponse, error) {
	if service == nil || nilIntakeExecutor(service.executor) ||
		ctx == nil || userID == uuid.Nil ||
		intake.ValidateIdempotencyKey(idempotencyKey) != nil ||
		contract.ValidateCreateSubmissionCommand(command) != nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid submission request",
			intake.ErrInvalidCommand,
		)
	}
	cid := correlation.FromContext(ctx)
	if correlation.Validate(cid) != nil {
		return nil, apperr.New(
			apperr.CodeBadRequest,
			"invalid request correlation",
			intake.ErrInvalidCommand,
		)
	}
	contextFields := []logger.Field{
		logger.String("user_id", userID.String()),
		logger.String("problem_id", command.ProblemID.String()),
		logger.String("runtime_key", command.RuntimeKey),
	}
	if command.ContestID != nil {
		contextFields = append(
			contextFields,
			logger.String("contest_id", command.ContestID.String()),
		)
	}
	ctx = logger.WithFields(ctx, contextFields...)
	logger.FromContext(ctx).Debug(
		"durable submission intake started",
		zap.Int("source_bytes", len(command.SourceCode)),
	)
	result, err := service.executor.Execute(ctx, intake.Command{
		UserID:    userID,
		ProblemID: command.ProblemID, ContestID: cloneSubmissionUUID(command.ContestID), ContestProblemID: cloneSubmissionUUID(command.ContestProblemID), RuntimeKey: command.RuntimeKey, Source: []byte(command.SourceCode),
		IdempotencyKey: idempotencyKey,
		CorrelationID:  cid,
	})
	if err != nil {
		fields := []zap.Field{
			zap.String("reason", intakeRejectionReason(err)),
			zap.Error(err),
		}
		logger.FromContext(ctx).Info(
			"durable submission intake rejected",
			fields...,
		)
		return nil, mapIntakeError(err)
	}
	if result.SubmissionID == uuid.Nil ||
		result.Generation != 1 ||
		result.Phase != entity.PhaseQueued ||
		result.AggregateVersion != 1 ||
		result.SubmittedAt.IsZero() ||
		correlation.Validate(result.CorrelationID) != nil ||
		(result.ContestSequence != nil && *result.ContestSequence < 1) {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"submission intake returned an invalid result",
			intake.ErrPersistence,
		)
	}
	response := &contract.SubmissionAcceptedResponse{
		ID:                    result.SubmissionID,
		AggregateVersion:      result.AggregateVersion,
		LatestGeneration:      result.Generation,
		LatestGenerationPhase: string(result.Phase),
		SubmittedAt:           result.SubmittedAt,
		ContestSubmissionSequence: cloneSubmissionInt64(
			result.ContestSequence,
		),
		CID:              result.CorrelationID,
		IdempotentReplay: result.ExactReplay,
	}
	logger.FromContext(ctx).Info(
		"durable submission intake accepted",
		zap.String("submission_id", result.SubmissionID.String()),
		zap.Int64("generation", result.Generation),
		zap.Bool("idempotent_replay", result.ExactReplay),
	)
	return response, nil
}

func intakeRejectionReason(err error) string {
	switch {
	case errors.Is(err, intake.ErrInvalidCommand):
		return "invalid_command"
	case errors.Is(err, intake.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, intake.ErrPrincipalIneligible):
		return "principal_ineligible"
	case errors.Is(err, intake.ErrAdmissionDenied):
		return "per_user_admission"
	case errors.Is(err, intake.ErrContestUnavailable):
		return "contest_unavailable"
	case errors.Is(err, intake.ErrContestParticipationDenied):
		return "contest_participation"
	case errors.Is(err, intake.ErrContestCapacityExhausted):
		return "contest_capacity"
	case errors.Is(err, intake.ErrContestProblemUnavailable):
		return "contest_problem"
	case errors.Is(err, intake.ErrProblemUnavailable):
		return "problem_unavailable"
	case errors.Is(err, intake.ErrCapabilityUnavailable):
		return "judge_capacity"
	case errors.Is(err, context.Canceled):
		return "request_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "request_deadline"
	default:
		return "persistence"
	}
}

func mapIntakeError(err error) error {
	switch {
	case errors.Is(err, intake.ErrInvalidCommand):
		return apperr.New(
			apperr.CodeBadRequest,
			"invalid submission request",
			err,
		)
	case errors.Is(err, intake.ErrIdempotencyConflict):
		return apperr.New(
			apperr.CodeConflict,
			"idempotency key was already used for another submission",
			err,
		)
	case errors.Is(err, intake.ErrPrincipalIneligible):
		return apperr.New(
			apperr.CodeForbidden,
			"account is not eligible to submit",
			err,
		)
	case errors.Is(err, intake.ErrAdmissionDenied):
		return apperr.New(
			apperr.CodeTooManyRequests,
			"too many active submissions",
			err,
		)
	case errors.Is(err, intake.ErrContestUnavailable):
		return apperr.New(
			apperr.CodeConflict,
			"contest is not accepting submissions",
			err,
		)
	case errors.Is(err, intake.ErrContestParticipationDenied):
		return apperr.New(
			apperr.CodeForbidden,
			"contest participation is not allowed",
			err,
		)
	case errors.Is(err, intake.ErrContestCapacityExhausted):
		return apperr.New(
			apperr.CodeConflict,
			"contest registration capacity is exhausted",
			err,
		)
	case errors.Is(err, intake.ErrContestProblemUnavailable):
		return apperr.New(
			apperr.CodeNotFound,
			"contest problem is unavailable",
			err,
		)
	case errors.Is(err, intake.ErrProblemUnavailable):
		return apperr.New(
			apperr.CodeNotFound,
			"problem is unavailable",
			err,
		)
	case errors.Is(err, intake.ErrCapabilityUnavailable):
		return apperr.New(
			ports.CodeJudgeCapacityUnavailable,
			"judge capacity is temporarily unavailable",
			err,
		)
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return apperr.New(
			apperr.CodeInternalServer,
			"submission intake failed",
			err,
		)
	}
}

func cloneSubmissionUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneSubmissionInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func nilIntakeExecutor(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ ports.SubmissionCommandService = (*submissionCommandService)(nil)
