package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const finalizationLeaseTokenBytes = 32

type FinalizationService struct {
	finalizer ports.SubmissionFinalizer
	observer  FinalizationMetricsObserver
}

type FinalizationMetricOutcome string

const (
	FinalizationMetricSuccess  FinalizationMetricOutcome = "success"
	FinalizationMetricRejected FinalizationMetricOutcome = "rejected"
	FinalizationMetricDead     FinalizationMetricOutcome = "dead"
	FinalizationMetricError    FinalizationMetricOutcome = "error"
)

// FinalizationMetricsObserver sees only a closed result class. Durable IDs,
// verdict detail, diagnostics, source, and lease capabilities remain private.
type FinalizationMetricsObserver interface {
	RecordFinalization(context.Context, FinalizationMetricOutcome)
}

func NewFinalizationService(
	finalizer ports.SubmissionFinalizer,
) (*FinalizationService, error) {
	return NewObservedFinalizationService(finalizer, nil)
}

func NewObservedFinalizationService(
	finalizer ports.SubmissionFinalizer,
	observer FinalizationMetricsObserver,
) (*FinalizationService, error) {
	if nilFinalizer(finalizer) {
		return nil, ports.ErrFinalizationUnavailable
	}
	return &FinalizationService{finalizer: finalizer, observer: observer}, nil
}

func (service *FinalizationService) FinalizeContestantEvaluation(
	ctx context.Context,
	request ports.FinalizationRequest,
) (ports.FinalizationResult, error) {
	if service == nil || nilFinalizer(service.finalizer) || ctx == nil ||
		validateContestantFinalization(ctx, request) != nil {
		if service != nil {
			service.recordFinalization(ctx, FinalizationMetricRejected)
		}
		return ports.FinalizationResult{}, ports.ErrInvalidFinalization
	}
	ctx = finalizationContext(ctx, request.Judgement.ID)
	log := logger.FromContext(ctx)
	log.Debug(
		"judge contestant finalization started",
		zap.String("verdict", string(request.Judgement.Verdict)),
		zap.Int("test_results", len(request.TestResults)),
	)
	result, err := service.finalizer.FinalizeContestantEvaluation(ctx, request)
	if err != nil {
		service.recordFinalizationError(ctx, err)
		log.Error("judge contestant finalization failed", zap.Error(err))
		return ports.FinalizationResult{}, err
	}
	if validateFinalizationResult(request.Lease, request.Judgement.ID, result) != nil {
		service.recordFinalization(ctx, FinalizationMetricRejected)
		log.Error(
			"judge contestant finalization returned invalid canonical state",
			zap.Error(ports.ErrFinalizationConflict),
		)
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	service.recordFinalization(ctx, FinalizationMetricSuccess)
	log.Info(
		"judge contestant finalization committed",
		zap.String("verdict", string(request.Judgement.Verdict)),
		zap.Int64("aggregate_version", result.AggregateVersion),
		zap.Bool("idempotent_replay", result.IdempotentReplay),
		zap.Bool("solve_transition", result.SolveTransitionID != nil),
	)
	return result, nil
}

func (service *FinalizationService) FinalizeSystemError(
	ctx context.Context,
	request ports.FatalGenerationRequest,
) (ports.FinalizationResult, error) {
	if service == nil || nilFinalizer(service.finalizer) || ctx == nil ||
		validateFatalFinalization(ctx, request) != nil {
		if service != nil {
			service.recordFinalization(ctx, FinalizationMetricRejected)
		}
		return ports.FinalizationResult{}, ports.ErrInvalidFinalization
	}
	ctx = finalizationContext(ctx, request.Judgement.ID)
	log := logger.FromContext(ctx)
	log.Debug(
		"judge system-error finalization started",
		zap.String("failure_classification", request.Failure.Classification),
		zap.String("failure_code", request.Failure.Code),
	)
	result, err := service.finalizer.FinalizeSystemError(ctx, request)
	if err != nil {
		service.recordFinalizationError(ctx, err)
		log.Error("judge system-error finalization failed", zap.Error(err))
		return ports.FinalizationResult{}, err
	}
	if validateFinalizationResult(request.Lease, request.Judgement.ID, result) != nil ||
		result.ActiveVerdictChanged || result.SolveTransitionID != nil {
		service.recordFinalization(ctx, FinalizationMetricRejected)
		log.Error(
			"judge system-error finalization returned invalid canonical state",
			zap.Error(ports.ErrFinalizationConflict),
		)
		return ports.FinalizationResult{}, ports.ErrFinalizationConflict
	}
	service.recordFinalization(ctx, FinalizationMetricDead)
	log.Warn(
		"judge system-error generation committed",
		zap.String("failure_classification", request.Failure.Classification),
		zap.String("failure_code", request.Failure.Code),
		zap.Int64("aggregate_version", result.AggregateVersion),
		zap.Bool("idempotent_replay", result.IdempotentReplay),
	)
	return result, nil
}

func (service *FinalizationService) recordFinalizationError(ctx context.Context, err error) {
	outcome := FinalizationMetricError
	if errors.Is(err, ports.ErrInvalidFinalization) ||
		errors.Is(err, ports.ErrFinalizationConflict) ||
		errors.Is(err, ports.ErrLeaseLost) ||
		errors.Is(err, ports.ErrLeaseCancelled) ||
		errors.Is(err, ports.ErrWorkerBootLost) {
		outcome = FinalizationMetricRejected
	}
	service.recordFinalization(ctx, outcome)
}

func (service *FinalizationService) recordFinalization(
	ctx context.Context,
	outcome FinalizationMetricOutcome,
) {
	if service == nil || service.observer == nil {
		return
	}
	defer func() { _ = recover() }()
	service.observer.RecordFinalization(ctx, outcome)
}

func validateContestantFinalization(
	ctx context.Context,
	request ports.FinalizationRequest,
) error {
	if validateFinalizationLease(ctx, request.Lease, request.ExpectedSubmissionVersion) != nil ||
		request.Judgement.Validate() != nil ||
		!request.Judgement.Verdict.Contestant() ||
		!judgementMatchesLease(request.Judgement, request.Lease) ||
		validateTestResults(request.Judgement, request.Lease, request.TestResults) != nil {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func validateFatalFinalization(
	ctx context.Context,
	request ports.FatalGenerationRequest,
) error {
	if validateFinalizationLease(ctx, request.Lease, request.ExpectedSubmissionVersion) != nil ||
		request.Judgement.Validate() != nil ||
		request.Judgement.Verdict != entity.VerdictSystemError ||
		!judgementMatchesLease(request.Judgement, request.Lease) ||
		!validFinalizationCode(request.Failure.Classification) ||
		!validFinalizationCode(request.Failure.Code) ||
		!validFinalizationText(request.Failure.Summary, 16<<10) {
		return ports.ErrInvalidFinalization
	}
	if request.Judgement.WinningAttemptID != nil &&
		*request.Judgement.WinningAttemptID != request.Lease.Attempt.ID {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func validateFinalizationLease(
	ctx context.Context,
	lease ports.JobLease,
	expectedVersion int64,
) error {
	cid := correlation.FromContext(ctx)
	if expectedVersion < 1 ||
		lease.Job.ID == uuid.Nil ||
		lease.Job.SubmissionID == uuid.Nil ||
		lease.Job.Generation < 1 ||
		lease.Job.JudgeSpecID == uuid.Nil ||
		lease.Attempt.ID == uuid.Nil ||
		lease.Attempt.JobID != lease.Job.ID ||
		lease.Attempt.SubmissionID != lease.Job.SubmissionID ||
		lease.Attempt.Generation != lease.Job.Generation ||
		lease.Attempt.JudgeSpecID != lease.Job.JudgeSpecID ||
		lease.Attempt.WorkerID == uuid.Nil ||
		lease.Attempt.WorkerBootID == uuid.Nil ||
		lease.JudgeSpec.ID != lease.Job.JudgeSpecID ||
		lease.JudgeSpec.SubmissionID != lease.Job.SubmissionID ||
		lease.JudgeSpec.Generation != lease.Job.Generation ||
		lease.LeaseOwner == "" ||
		len(lease.LeaseOwner) > 128 ||
		len(lease.LeaseToken) != finalizationLeaseTokenBytes ||
		expectedVersion < lease.SubmissionVersion+2 ||
		expectedVersion > lease.SubmissionVersion+3 ||
		correlation.Validate(cid) != nil ||
		cid != lease.Job.OriginCID {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func judgementMatchesLease(
	judgement entity.Judgement,
	lease ports.JobLease,
) bool {
	specChecksum, specChecksumOK := checksum32(
		lease.JudgeSpec.SpecChecksum,
	)
	rewardChecksum, rewardChecksumOK := checksum32(
		lease.JudgeSpec.RewardSnapshotChecksum,
	)
	return specChecksumOK && rewardChecksumOK &&
		judgement.SubmissionID == lease.Job.SubmissionID &&
		judgement.Generation == lease.Job.Generation &&
		judgement.JudgeSpecID == lease.Job.JudgeSpecID &&
		judgement.ProblemRevisionID == lease.JudgeSpec.ProblemRevisionID &&
		judgement.TestsetRevisionID == lease.JudgeSpec.TestsetRevisionID &&
		judgement.JudgeSpecChecksum == specChecksum &&
		judgement.RewardSnapshotChecksum == rewardChecksum &&
		judgement.RuntimeProfileVersion == lease.JudgeSpec.RuntimeProfileVersion &&
		judgement.CompilerVersion == lease.JudgeSpec.CompilerVersion &&
		judgement.RuntimeBootstrapVersion == lease.JudgeSpec.RuntimeBootstrapVersion &&
		judgement.SupervisorProtocolVersion == lease.JudgeSpec.SupervisorProtocol &&
		judgement.SandboxImageDigest == lease.JudgeSpec.SandboxImageDigest &&
		judgement.ArtifactManifestChecksum == lease.JudgeSpec.ArtifactManifestChecksum &&
		judgement.ReleaseID == lease.JudgeSpec.ReleaseID &&
		judgement.ApprovedReleaseManifestChecksum == lease.JudgeSpec.ApprovedManifestChecksum &&
		judgement.CheckerImplementationChecksum == lease.JudgeSpec.CheckerImplementationChecksum &&
		judgement.OriginCID == lease.Job.OriginCID
}

func validateTestResults(
	judgement entity.Judgement,
	lease ports.JobLease,
	results []ports.JudgeTestResult,
) error {
	if len(results) > problemcontract.MaximumTestCases {
		return ports.ErrInvalidFinalization
	}
	if judgement.Verdict == entity.VerdictCompileError {
		if len(results) != 0 || judgement.PassedCount != 0 {
			return ports.ErrInvalidFinalization
		}
		return nil
	}
	if judgement.TotalCount < 1 || len(results) == 0 {
		return ports.ErrInvalidFinalization
	}
	ordered := append([]ports.JudgeTestResult(nil), results...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].ExecutionOrder < ordered[right].ExecutionOrder
	})
	seenCases := make(map[uuid.UUID]struct{}, len(ordered))
	seenExecutionOrder := make(map[int]struct{}, len(ordered))
	seenGroupCaseOrder := make(map[[2]int]struct{}, len(ordered))
	accepted := 0
	hasTerminalVerdict := false
	previousOrder := -1
	for _, result := range ordered {
		if result.JudgeSpecID != lease.Job.JudgeSpecID ||
			result.TestsetRevisionID != lease.JudgeSpec.TestsetRevisionID ||
			result.TestcaseRevisionID == uuid.Nil ||
			result.GroupID == uuid.Nil ||
			result.GroupIndex < 0 ||
			result.GroupIndex >= problemcontract.MaximumTestcaseGroups ||
			result.CaseOrder < 0 ||
			result.CaseOrder >= problemcontract.MaximumTestCases ||
			result.ExecutionOrder < 0 ||
			result.ExecutionOrder >= problemcontract.MaximumTestCases ||
			result.ExecutionOrder <= previousOrder ||
			!validTestVerdict(result.Verdict) ||
			result.TimeMilliseconds < 0 ||
			result.TimeMilliseconds > problemcontract.MaximumWallTimeLimitMilliseconds ||
			result.MemoryKilobytes < 0 ||
			result.MemoryKilobytes > problemcontract.MaximumMemoryLimitKilobytes ||
			result.StdoutBytes < 0 ||
			result.StdoutBytes > problemcontract.MaximumOutputLimitBytes ||
			result.StderrBytes < 0 ||
			result.StderrBytes > problemcontract.MaximumOutputLimitBytes ||
			result.ExecutionStartedAt.IsZero() ||
			result.ExecutionEndedAt.Before(result.ExecutionStartedAt) ||
			(result.ExitCode != nil && result.TerminatingSignal != nil) ||
			(result.TerminatingSignal != nil && *result.TerminatingSignal < 1) ||
			!validCheckerMetadata(result.CheckerMetadata) {
			return ports.ErrInvalidFinalization
		}
		if result.Kind != problemcontract.TestKindSample &&
			result.Kind != problemcontract.TestKindHidden {
			return ports.ErrInvalidFinalization
		}
		if result.SafeDiagnostic != nil &&
			(result.Kind != problemcontract.TestKindSample ||
				!validFinalizationText(*result.SafeDiagnostic, 16<<10)) {
			return ports.ErrInvalidFinalization
		}
		if _, duplicate := seenCases[result.TestcaseRevisionID]; duplicate {
			return ports.ErrInvalidFinalization
		}
		seenCases[result.TestcaseRevisionID] = struct{}{}
		if _, duplicate := seenExecutionOrder[result.ExecutionOrder]; duplicate {
			return ports.ErrInvalidFinalization
		}
		seenExecutionOrder[result.ExecutionOrder] = struct{}{}
		groupCaseOrder := [2]int{result.GroupIndex, result.CaseOrder}
		if _, duplicate := seenGroupCaseOrder[groupCaseOrder]; duplicate {
			return ports.ErrInvalidFinalization
		}
		seenGroupCaseOrder[groupCaseOrder] = struct{}{}
		previousOrder = result.ExecutionOrder
		switch result.Verdict {
		case entity.VerdictAccepted:
			accepted++
		case judgement.Verdict:
			hasTerminalVerdict = true
		}
	}
	if accepted != judgement.PassedCount {
		return ports.ErrInvalidFinalization
	}
	if judgement.Verdict == entity.VerdictAccepted {
		if len(results) != judgement.TotalCount ||
			accepted != judgement.TotalCount {
			return ports.ErrInvalidFinalization
		}
		return nil
	}
	if !hasTerminalVerdict {
		return ports.ErrInvalidFinalization
	}
	return nil
}

func validTestVerdict(verdict entity.Verdict) bool {
	switch verdict {
	case entity.VerdictAccepted,
		entity.VerdictWrongAnswer,
		entity.VerdictRuntimeError,
		entity.VerdictTimeLimitExceeded,
		entity.VerdictMemoryLimitExceeded,
		entity.VerdictProcessLimitExceeded,
		entity.VerdictOutputLimitExceeded:
		return true
	default:
		return false
	}
}

func validCheckerMetadata(value json.RawMessage) bool {
	if len(value) == 0 || len(value) > problemcontract.MaximumCheckerConfigBytes ||
		!json.Valid(value) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(value, &object) == nil && object != nil
}

func validFinalizationCode(value string) bool {
	if len(value) == 0 || len(value) > 64 ||
		value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validFinalizationText(value string, maximumBytes int) bool {
	if len(value) == 0 || len(value) > maximumBytes ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character == '\n' || character == '\r' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func validateFinalizationResult(
	lease ports.JobLease,
	judgementID uuid.UUID,
	result ports.FinalizationResult,
) error {
	if result.SubmissionID != lease.Job.SubmissionID ||
		result.JudgementID != judgementID ||
		result.AggregateVersion < 1 ||
		result.FinalizedAt.IsZero() ||
		result.State.ID != lease.Job.SubmissionID ||
		result.State.AggregateVersion < result.AggregateVersion ||
		result.State.Validate() != nil ||
		(result.SolveTransitionID != nil &&
			*result.SolveTransitionID == uuid.Nil) {
		return ports.ErrFinalizationConflict
	}
	return nil
}

func checksum32(value []byte) ([32]byte, bool) {
	var result [32]byte
	if len(value) != len(result) {
		return result, false
	}
	copy(result[:], value)
	return result, true
}

func finalizationContext(
	ctx context.Context,
	judgementID uuid.UUID,
) context.Context {
	return logger.WithFields(
		ctx,
		logger.String("judgement_id", judgementID.String()),
	)
}

func nilFinalizer(value any) bool {
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

var _ ports.SubmissionFinalizer = (*FinalizationService)(nil)
