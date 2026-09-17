package submissionintake

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"

	contestentity "github.com/huynhanx03/judgify/internal/contest/domain"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	contestports "github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
)

const (
	commandVersion                      = "judgify.submission-intake-command.v2"
	maximumTotalAttemptWallMilliseconds = int64(24 * time.Hour / time.Millisecond)
)

var (
	ErrInvalidCommand             = errors.New("submission intake: invalid command")
	ErrIdempotencyConflict        = errors.New("submission intake: idempotency conflict")
	ErrPrincipalIneligible        = errors.New("submission intake: principal ineligible")
	ErrAdmissionDenied            = errors.New("submission intake: admission denied")
	ErrContestUnavailable         = errors.New("submission intake: contest unavailable")
	ErrContestParticipationDenied = errors.New("submission intake: contest participation denied")
	ErrContestCapacityExhausted   = errors.New("submission intake: contest capacity exhausted")
	ErrContestProblemUnavailable  = errors.New("submission intake: contest problem unavailable")
	ErrProblemUnavailable         = errors.New("submission intake: problem unavailable")
	ErrCapabilityUnavailable      = errors.New("submission intake: capability unavailable")
	ErrPersistence                = errors.New("submission intake: persistence failed")
	runtimeKeyPattern             = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

type Command struct {
	UserID           uuid.UUID
	ProblemID        uuid.UUID
	ContestID        *uuid.UUID
	ContestProblemID *uuid.UUID
	RuntimeKey       string
	Source           []byte
	IdempotencyKey   string
	CorrelationID    string
}

type Result struct {
	SubmissionID     uuid.UUID
	Generation       int64
	Phase            entity.Phase
	AggregateVersion int64
	SubmittedAt      time.Time
	ContestSequence  *int64
	CorrelationID    string
	ExactReplay      bool
}

type CommandIdentity struct {
	UserID         uuid.UUID
	IdempotencyKey string
	RequestHash    [32]byte
}

type ProblemRequest struct {
	ProblemID                  uuid.UUID
	ExpectedProblemRevisionID  *uuid.UUID
	ExpectedTestsetRevisionID  *uuid.UUID
	ExpectedCheckerKey         string
	ExpectedCheckerVersion     string
	ExpectedCheckerConfig      json.RawMessage
	ExpectedAllowedRuntimeKeys []string
	ExpectedSnapshotChecksum   *[32]byte
	RuntimeKey                 string
}

type ProblemSnapshot struct {
	ProblemID              uuid.UUID
	ProblemRevisionID      uuid.UUID
	TestsetRevisionID      uuid.UUID
	CheckerKey             string
	CheckerVersion         string
	CheckerConfig          json.RawMessage
	AllowedRuntimeKeys     []string
	RewardSnapshot         json.RawMessage
	RewardSnapshotChecksum [32]byte
	CPUTimeMilliseconds    int
	WallTimeMilliseconds   int
	MemoryKilobytes        int
	OutputBytes            int
	ProcessCount           int
	TestCount              int
}

type CapabilityRequest struct {
	RuntimeKey     string
	CheckerKey     string
	CheckerVersion string
}

type CapabilitySnapshot struct {
	ReleaseID                       string
	ApprovedReleaseManifestChecksum string
	Language                        string
	JudgeSpecProtocol               string
	RuntimeProfileVersion           string
	CompilerVersion                 string
	RuntimeBootstrapVersion         string
	SandboxImageDigest              string
	CompileCommandID                string
	RunCommandID                    string
	ArtifactManifestVersion         string
	ArtifactManifestChecksum        string
	ArtifactManifest                json.RawMessage
	SupervisorProtocolVersion       string
	CapabilityProfileHash           string
	CheckerImplementationChecksum   string
	CompileCPUTimeMilliseconds      int
	CompileWallTimeMilliseconds     int
	TotalAttemptWallMilliseconds    int
}

type AcceptedSubmission struct {
	Command      Command
	RequestHash  [32]byte
	SourceSHA256 [32]byte
	SubmittedAt  time.Time
	Contest      *contestports.ContestSubmissionReservation
	Problem      ProblemSnapshot
	Capability   CapabilitySnapshot
}

type WakeupHint struct {
	SubmissionID  uuid.UUID
	Generation    int64
	CorrelationID string
}

type CommandGate interface {
	LockAndReplay(context.Context, CommandIdentity) (*Result, error)
}

type IdentityEligibility interface {
	RequireEligible(context.Context, uuid.UUID) error
}

type SubmissionAdmission interface {
	Reserve(context.Context, uuid.UUID) error
}

type ContestReservation interface {
	ReserveContestSubmission(
		context.Context,
		contestports.ReserveContestSubmissionRequest,
	) (contestports.ContestSubmissionReservation, error)
}

type ProblemResolver interface {
	Resolve(context.Context, ProblemRequest) (ProblemSnapshot, error)
}

type CapabilityResolver interface {
	Resolve(context.Context, CapabilityRequest) (CapabilitySnapshot, error)
}

type DatabaseClock interface {
	Now(context.Context) (time.Time, error)
}

type Writer interface {
	Persist(context.Context, AcceptedSubmission) (Result, error)
}

// Wakeup is a disposable local latency hint. Implementations may signal a
// database claimant but must never be required for correctness.
type Wakeup interface {
	Wake(context.Context, WakeupHint) error
}

type Dependencies struct {
	Transactions platformtx.UnitOfWork
	Gate         CommandGate
	Identity     IdentityEligibility
	Admission    SubmissionAdmission
	Contest      ContestReservation
	Problem      ProblemResolver
	Capability   CapabilityResolver
	Clock        DatabaseClock
	Writer       Writer
	Wakeup       Wakeup
}

type Workflow struct {
	dependencies Dependencies
}

func New(dependencies Dependencies) (*Workflow, error) {
	if nilDependency(dependencies.Transactions) ||
		nilDependency(dependencies.Gate) ||
		nilDependency(dependencies.Identity) ||
		nilDependency(dependencies.Admission) ||
		nilDependency(dependencies.Contest) ||
		nilDependency(dependencies.Problem) ||
		nilDependency(dependencies.Capability) ||
		nilDependency(dependencies.Clock) ||
		nilDependency(dependencies.Writer) {
		return nil, ErrInvalidCommand
	}
	return &Workflow{dependencies: dependencies}, nil
}

func (workflow *Workflow) Execute(ctx context.Context, command Command) (Result, error) {
	if workflow == nil || ctx == nil {
		return Result{}, ErrInvalidCommand
	}
	requestHash, err := HashCommand(command)
	if err != nil {
		return Result{}, err
	}
	command.RuntimeKey = strings.Clone(command.RuntimeKey)
	command.Source = append([]byte(nil), command.Source...)
	command.IdempotencyKey = strings.Clone(command.IdempotencyKey)
	command.CorrelationID = strings.Clone(command.CorrelationID)

	var result Result
	err = workflow.dependencies.Transactions.Within(ctx, func(txCtx context.Context) error {
		replay, gateErr := workflow.dependencies.Gate.LockAndReplay(txCtx, CommandIdentity{
			UserID: command.UserID, IdempotencyKey: command.IdempotencyKey,
			RequestHash: requestHash,
		})
		if gateErr != nil {
			return classifyGateError(gateErr)
		}
		if replay != nil {
			result = *replay
			result.ExactReplay = true
			return nil
		}
		if eligibilityErr := workflow.dependencies.Identity.RequireEligible(txCtx, command.UserID); eligibilityErr != nil {
			return classifyParticipantError(eligibilityErr, ErrPrincipalIneligible)
		}
		if admissionErr := workflow.dependencies.Admission.Reserve(txCtx, command.UserID); admissionErr != nil {
			return classifyParticipantError(admissionErr, ErrAdmissionDenied)
		}

		problemRequest := ProblemRequest{
			ProblemID: command.ProblemID, RuntimeKey: command.RuntimeKey,
		}
		var contestReservation *contestports.ContestSubmissionReservation
		if command.ContestID != nil {
			reserved, reserveErr := workflow.dependencies.Contest.ReserveContestSubmission(
				txCtx,
				contestports.ReserveContestSubmissionRequest{
					ContestID: *command.ContestID, ContestProblemID: *command.ContestProblemID,
					ProblemID: command.ProblemID, UserID: command.UserID,
					RuntimeKey: command.RuntimeKey,
				},
			)
			if reserveErr != nil {
				return classifyContestError(reserveErr)
			}
			contestReservation = &reserved
			problemRequest.ExpectedProblemRevisionID = &reserved.ProblemRevisionID
			problemRequest.ExpectedTestsetRevisionID = &reserved.TestsetRevisionID
			problemRequest.ExpectedCheckerKey = reserved.CheckerKey
			problemRequest.ExpectedCheckerVersion = reserved.CheckerVersion
			problemRequest.ExpectedCheckerConfig = append(
				json.RawMessage(nil), reserved.CheckerConfig...,
			)
			problemRequest.ExpectedAllowedRuntimeKeys = append(
				[]string(nil), reserved.AllowedRuntimeKeys...,
			)
			checksum := reserved.ProblemSnapshotChecksum
			problemRequest.ExpectedSnapshotChecksum = &checksum
		}

		problem, problemErr := workflow.dependencies.Problem.Resolve(txCtx, problemRequest)
		if problemErr != nil {
			return classifyParticipantError(problemErr, ErrProblemUnavailable)
		}
		capability, capabilityErr := workflow.dependencies.Capability.Resolve(
			txCtx,
			CapabilityRequest{
				RuntimeKey: command.RuntimeKey,
				CheckerKey: problem.CheckerKey, CheckerVersion: problem.CheckerVersion,
			},
		)
		if capabilityErr != nil {
			return classifyParticipantError(capabilityErr, ErrCapabilityUnavailable)
		}
		language, supported := judging.ParseLanguage(capability.Language)
		if !supported || string(language) != capability.Language {
			return ErrCapabilityUnavailable
		}
		totalAttemptWall, totalErr := totalAttemptWallMilliseconds(problem, capability)
		if totalErr != nil {
			return totalErr
		}
		capability.TotalAttemptWallMilliseconds = totalAttemptWall

		var submittedAt time.Time
		if contestReservation != nil {
			submittedAt = contestReservation.SubmittedAt
		} else {
			submittedAt, err = workflow.dependencies.Clock.Now(txCtx)
			if err != nil {
				return classifyParticipantError(err, ErrPersistence)
			}
		}
		if submittedAt.IsZero() {
			return ErrPersistence
		}
		result, err = workflow.dependencies.Writer.Persist(txCtx, AcceptedSubmission{
			Command: command, RequestHash: requestHash,
			SourceSHA256: sha256.Sum256(command.Source), SubmittedAt: submittedAt.UTC(),
			Contest: contestReservation, Problem: problem, Capability: capability,
		})
		if err != nil {
			return classifyParticipantError(err, ErrPersistence)
		}
		workflow.registerBestEffortWakeup(txCtx, result)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (workflow *Workflow) registerBestEffortWakeup(ctx context.Context, result Result) {
	if workflow == nil || nilDependency(workflow.dependencies.Wakeup) ||
		result.SubmissionID == uuid.Nil || result.ExactReplay {
		return
	}
	hint := WakeupHint{
		SubmissionID: result.SubmissionID, Generation: result.Generation,
		CorrelationID: result.CorrelationID,
	}
	err := platformtx.AfterCommit(ctx, func(callbackContext context.Context) error {
		wakeContext := wakeupLogContext(callbackContext, hint)
		if wakeErr := workflow.dependencies.Wakeup.Wake(wakeContext, hint); wakeErr != nil {
			logger.FromContext(wakeContext).Debug(
				"submission claimant wakeup hint failed",
				zap.Error(wakeErr),
			)
		}
		return nil
	})
	if err != nil {
		logger.FromContext(wakeupLogContext(ctx, hint)).Debug(
			"submission claimant wakeup hint was not registered",
			zap.Error(err),
		)
	}
}

func wakeupLogContext(ctx context.Context, hint WakeupHint) context.Context {
	ctx = correlation.WithContext(ctx, hint.CorrelationID)
	return logger.WithFields(
		ctx,
		logger.String("submission_id", hint.SubmissionID.String()),
		logger.String("generation", strconv.FormatInt(hint.Generation, 10)),
	)
}

func HashCommand(command Command) ([32]byte, error) {
	if command.UserID == uuid.Nil ||
		command.ProblemID == uuid.Nil ||
		!runtimeKeyPattern.MatchString(command.RuntimeKey) ||
		(command.ContestID == nil) != (command.ContestProblemID == nil) ||
		(command.ContestID != nil &&
			(*command.ContestID == uuid.Nil || *command.ContestProblemID == uuid.Nil)) ||
		ValidateIdempotencyKey(command.IdempotencyKey) != nil ||
		correlation.Validate(command.CorrelationID) != nil ||
		judging.ValidateSourceCode(string(command.Source)) != nil {
		return [32]byte{}, ErrInvalidCommand
	}
	digest := sha256.New()
	writeHashPart(digest, []byte(commandVersion))
	writeHashPart(digest, command.UserID[:])
	writeHashPart(digest, command.ProblemID[:])
	if command.ContestID == nil {
		writeHashPart(digest, nil)
		writeHashPart(digest, nil)
	} else {
		writeHashPart(digest, command.ContestID[:])
		writeHashPart(digest, command.ContestProblemID[:])
	}
	writeHashPart(digest, []byte(command.RuntimeKey))
	sourceHash := sha256.Sum256(command.Source)
	writeHashPart(digest, sourceHash[:])
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}

func ValidateIdempotencyKey(value string) error {
	if len(value) < 1 ||
		len(value) > 128 ||
		!utf8.ValidString(value) ||
		value != strings.TrimSpace(value) {
		return ErrInvalidCommand
	}
	for _, character := range value {
		if character < 0x20 ||
			character > 0x7e ||
			character > unicode.MaxASCII ||
			unicode.IsControl(character) {
			return ErrInvalidCommand
		}
	}
	return nil
}

func totalAttemptWallMilliseconds(
	problem ProblemSnapshot,
	capability CapabilitySnapshot,
) (int, error) {
	if problem.TestCount < 1 ||
		problem.WallTimeMilliseconds < 1 ||
		capability.CompileWallTimeMilliseconds < 1 ||
		capability.CompileCPUTimeMilliseconds < 1 ||
		capability.CompileCPUTimeMilliseconds > capability.CompileWallTimeMilliseconds {
		return 0, ErrCapabilityUnavailable
	}
	total := int64(capability.CompileWallTimeMilliseconds) +
		int64(problem.WallTimeMilliseconds)*int64(problem.TestCount)
	if total < int64(capability.CompileWallTimeMilliseconds) ||
		total > maximumTotalAttemptWallMilliseconds {
		return 0, ErrCapabilityUnavailable
	}
	return int(total), nil
}

func classifyGateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrIdempotencyConflict) ||
		errors.Is(err, ErrInvalidCommand) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errors.Join(ErrPersistence, err)
}

func classifyParticipantError(err error, logical error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, logical) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errors.Join(ErrPersistence, err)
}

func classifyContestError(err error) error {
	switch {
	case errors.Is(err, contestentity.ErrContestNotRunning),
		errors.Is(err, contestentity.ErrContestNotFound):
		return errors.Join(ErrContestUnavailable, err)
	case errors.Is(err, contestentity.ErrContestRegistrationRequired),
		errors.Is(err, contestentity.ErrContestRegistrationBanned),
		errors.Is(err, contestentity.ErrContestPolicy):
		return errors.Join(ErrContestParticipationDenied, err)
	case errors.Is(err, contestentity.ErrContestRegistrationFull):
		return errors.Join(ErrContestCapacityExhausted, err)
	case errors.Is(err, contestentity.ErrContestProblemUnavailable):
		return errors.Join(ErrContestProblemUnavailable, err)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errors.Join(ErrPersistence, err)
	}
}

func writeHashPart(digest hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = digest.Write(value)
}

func nilDependency(value any) bool {
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
