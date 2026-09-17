// Package ports defines Judgify's stable internal capability catalog.
package ports

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	commondto "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

// Submission capabilities are added here during the Submission migration.

var (
	ErrInvalidFinalization     = errors.New("submission finalization: invalid command")
	ErrFinalizationConflict    = errors.New("submission finalization: canonical result conflict")
	ErrFinalizationUnavailable = errors.New("submission finalization: unavailable")
)

type JudgeTestResult struct {
	JudgeSpecID        uuid.UUID
	TestsetRevisionID  uuid.UUID
	TestcaseRevisionID uuid.UUID
	GroupID            uuid.UUID
	GroupIndex         int
	// CaseOrder is immutable order inside the testcase's group and is bound
	// to testcase_revisions.order_index by an exact composite foreign key.
	CaseOrder int
	// ExecutionOrder is the attempt-global deterministic execution sequence.
	ExecutionOrder     int
	Kind               string
	StopOnFailure      bool
	Verdict            entity.Verdict
	TimeMilliseconds   int
	MemoryKilobytes    int
	ExitCode           *int
	TerminatingSignal  *int
	StdoutBytes        int
	StderrBytes        int
	SafeDiagnostic     *string
	CheckerMetadata    json.RawMessage
	ExecutionStartedAt time.Time
	ExecutionEndedAt   time.Time
}

type FinalizationRequest struct {
	Lease                     JobLease
	ExpectedSubmissionVersion int64
	// Judgement carries the worker's bounded result and claimed immutable
	// provenance. Persistence rechecks every copied fact against judge_specs
	// and replaces CompletedAt/OriginCID with database-owned values.
	Judgement   entity.Judgement
	TestResults []JudgeTestResult
}

type FatalGenerationRequest struct {
	Lease                     JobLease
	ExpectedSubmissionVersion int64
	Judgement                 entity.Judgement
	Failure                   InfrastructureFailure
}

type FinalizationResult struct {
	SubmissionID         uuid.UUID
	JudgementID          uuid.UUID
	AggregateVersion     int64
	ActiveVerdictChanged bool
	SolveTransitionID    *uuid.UUID
	FinalizedAt          time.Time
	IdempotentReplay     bool
	State                entity.SubmissionState
}

// SubmissionFinalizer persists attempt results, immutable judgement, active
// state, solve projection/transition, job state, and outbox rows atomically.
type SubmissionFinalizer interface {
	FinalizeContestantEvaluation(
		ctx context.Context,
		request FinalizationRequest,
	) (FinalizationResult, error)
	FinalizeSystemError(
		ctx context.Context,
		request FatalGenerationRequest,
	) (FinalizationResult, error)
}

// SolveTransitionReader is a bounded immutable catch-up port for downstream
// consumers that observe an aggregate-version gap.
type SolveTransitionReader interface {
	ReadAfter(
		ctx context.Context,
		solveID uuid.UUID,
		afterVersion int64,
		limit int,
	) ([]entity.SolveTransition, error)
}

var (
	ErrClaimDenied        = errors.New("submission jobs: claim denied")
	ErrWorkerBootLost     = errors.New("submission jobs: worker boot lost")
	ErrLeaseLost          = errors.New("submission jobs: lease lost")
	ErrLeaseCancelled     = errors.New("submission jobs: generation cancelled")
	ErrJudgePlanIntegrity = errors.New("submission jobs: judge plan integrity failure")
	ErrJudgeCaseIntegrity = errors.New("submission jobs: judge case integrity failure")
)

// JudgePlan is the checksum-verified execution contract loaded only after a
// durable claim. It contains no mutable Problem repository handles.
type JudgePlan struct {
	JobID             uuid.UUID
	SubmissionID      uuid.UUID
	JudgeSpecID       uuid.UUID
	Generation        int64
	UserID            uuid.UUID
	ProblemID         uuid.UUID
	ProblemRevisionID uuid.UUID
	TestsetRevisionID uuid.UUID
	Language          string
	JudgeSpecProtocol string
	Release           ReleaseSnapshot
	Runtime           RuntimeSnapshot
	Checker           CheckerSnapshot
	RewardFacts       json.RawMessage
	Limits            JudgeLimits
	SourceCode        string
	SourceSHA256      [32]byte
	SpecChecksum      [32]byte
	RewardChecksum    [32]byte
	TestsetChecksum   [32]byte
	Tests             []JudgeTestSpec
	OriginCID         string
}

// ReleaseSnapshot binds an execution plan to the reviewed release approval
// that authorized its exact checker implementation.
type ReleaseSnapshot struct {
	ID                            string
	ApprovedManifestChecksum      string
	CheckerImplementationChecksum string
}

type RuntimeSnapshot struct {
	Key                      string
	ProfileVersion           string
	CompilerVersion          string
	BootstrapVersion         string
	SandboxImageDigest       string
	CompileCommandID         string
	RunCommandID             string
	ArtifactManifestVersion  string
	ArtifactManifestChecksum string
	ArtifactManifest         json.RawMessage
	SupervisorProtocol       string
	CapabilityProfileHash    string
}

type CheckerSnapshot struct {
	Key                    string
	Version                string
	ImplementationChecksum string
	Config                 json.RawMessage
}

type JudgeLimits struct {
	CPUTimeMilliseconds          int
	WallTimeMilliseconds         int
	MemoryKilobytes              int
	OutputBytes                  int
	ProcessCount                 int
	CompileCPUTimeMilliseconds   int
	CompileWallTimeMilliseconds  int
	TotalAttemptWallMilliseconds int
}

type JudgeTestSpec struct {
	RevisionID     uuid.UUID
	TestCaseID     uuid.UUID
	GroupID        uuid.UUID
	GroupIndex     int
	OrderIndex     int
	Kind           string
	InputAssetID   uuid.UUID
	OutputAssetID  uuid.UUID
	InputSHA256    [32]byte
	OutputSHA256   [32]byte
	InputEncoding  string
	OutputEncoding string
	InputBytes     int64
	OutputBytes    int64
	StopOnFailure  bool
}

// JudgeAssetSnapshot is encoded, content-addressed test data loaded only by a
// lease-fenced worker. It is deliberately not a transport DTO and must never
// be serialized, logged, or copied into a contestant namespace unless it is
// the input asset for that exact testcase.
type JudgeAssetSnapshot struct {
	ID               uuid.UUID
	SHA256           [32]byte
	Encoding         string
	Content          []byte
	UncompressedSize int64
}

type JudgeCasePayload struct {
	TestcaseRevisionID uuid.UUID
	Input              JudgeAssetSnapshot
	Expected           JudgeAssetSnapshot
}

// JudgeSpecClaim is the complete immutable compatibility and provenance row
// selected by the claim transaction. No list or version range is represented.
type JudgeSpecClaim struct {
	ID                            uuid.UUID
	SubmissionID                  uuid.UUID
	Generation                    int64
	UserID                        uuid.UUID
	ProblemRevisionID             uuid.UUID
	TestsetRevisionID             uuid.UUID
	JudgeSpecProtocol             string
	RuntimeKey                    string
	RuntimeProfileVersion         string
	CompilerVersion               string
	RuntimeBootstrapVersion       string
	CompileCommandID              string
	RunCommandID                  string
	SandboxImageDigest            string
	ArtifactManifestVersion       string
	ArtifactManifestChecksum      string
	SupervisorProtocol            string
	CheckerKey                    string
	CheckerVersion                string
	ReleaseID                     string
	ApprovedManifestChecksum      string
	CheckerImplementationChecksum string
	CapabilityProfileHash         string
	SpecChecksum                  []byte
	RewardSnapshotChecksum        []byte
}

// JudgeSpecReader verifies the active lease and every referenced checksum.
// leaseToken is an opaque in-memory capability.
type JudgeSpecReader interface {
	ReadJudgePlan(ctx context.Context, lease JobLease) (JudgePlan, error)
}

// JudgeCaseReader resolves exactly one case from the immutable judge spec
// while re-checking the active lease. Expected bytes remain in the trusted
// worker process and never cross the Supervisor boundary.
type JudgeCaseReader interface {
	ReadJudgeCase(
		ctx context.Context,
		lease JobLease,
		testcaseRevisionID uuid.UUID,
	) (JudgeCasePayload, error)
}

type JobClaimRequest struct {
	WorkerID            uuid.UUID
	WorkerBootID        uuid.UUID
	LeaseOwner          string
	LeaseDuration       time.Duration
	HeartbeatFreshness  time.Duration
	Capacity            int
	GlobalRunningLimit  int
	PerUserRunningLimit int
}

type JobLease struct {
	Job               entity.JudgeJob
	Attempt           entity.JudgeAttempt
	JudgeSpec         JudgeSpecClaim
	LeaseOwner        string
	LeaseToken        []byte
	LeaseUntil        time.Time
	SubmissionVersion int64
	BasePriority      int
	CreatedAt         time.Time
}

type WorkerBootHeartbeat struct {
	WorkerID     uuid.UUID
	WorkerBootID uuid.UUID
}

type LeaseHeartbeat struct {
	JobID        uuid.UUID
	AttemptID    uuid.UUID
	WorkerID     uuid.UUID
	WorkerBootID uuid.UUID
	LeaseOwner   string
	LeaseToken   []byte
	ExtendTo     time.Time
}

type ReapRequest struct {
	Limit        int
	RetryInitial time.Duration
	RetryMaximum time.Duration
}

type ReapResult struct {
	Promoted  int
	Retried   int
	Dead      int
	Cancelled int
}

// JudgeJobRepository is the sole durable scheduling authority. Raw token
// bytes may be compared in constant time but must never be persisted or logged.
type JudgeJobRepository interface {
	Claim(ctx context.Context, request JobClaimRequest) ([]JobLease, error)
	HeartbeatWorkerBoot(ctx context.Context, heartbeat WorkerBootHeartbeat) error
	Heartbeat(
		ctx context.Context,
		heartbeat LeaseHeartbeat,
	) (acceptedLeaseUntil time.Time, err error)
	ReapExpired(ctx context.Context, request ReapRequest) (ReapResult, error)
	// AcknowledgeCancellation closes a deliberately cancelled in-flight lease
	// after its worker has stopped local execution. It is fenced by the lease
	// capability, idempotent for the same attempt, and releases capacity once.
	AcknowledgeCancellation(ctx context.Context, lease JobLease) error
	TransitionPhase(
		ctx context.Context,
		lease JobLease,
		expectedSubmissionVersion int64,
		next entity.Phase,
	) (entity.SubmissionState, error)
	RecordRetry(ctx context.Context, lease JobLease, failure InfrastructureFailure) error
}

type InfrastructureFailure struct {
	Classification string
	Code           string
	Summary        string
	RetryAfter     time.Duration
}

// InfrastructureIncident is the worker-observed, bounded failure description.
// It deliberately has no scheduling timestamp: retry timing belongs to the
// durable job service so every executor uses one reviewed backoff policy.
type InfrastructureIncident struct {
	Classification string
	Code           string
	Summary        string
}

var (
	// ErrOperationConflict is returned when an optimistic operational command
	// no longer matches the durable submission generation it observed.
	ErrOperationConflict = errors.New("submission operations: state conflict")
	// ErrOperationUnavailable keeps HTTP adapters from treating a missing
	// operation composition as an accidental successful no-op.
	ErrOperationUnavailable = errors.New("submission operations: unavailable")
	// ErrInvalidMySubmissionPage rejects malformed keyset queries or adapter
	// projections before they can become an ambiguous public response.
	ErrInvalidMySubmissionPage = errors.New("submission: invalid owner history page")
)

// CodeJudgeCapacityUnavailable is a Judgify application code rendered as a
// retryable HTTP 503 by the submission command adapter.
const (
	CodeJudgeCapacityUnavailable = 50301
	DefaultMySubmissionPageLimit = 20
	MaximumMySubmissionPageLimit = 50
)

// SubmissionCursor is the total descending order boundary for an owner's
// problem history.
type SubmissionCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// MySubmissionQuery is storage-ready: services normalize the optional HTTP
// request before passing this closed contract to an adapter.
type MySubmissionQuery struct {
	UserID    uuid.UUID
	ProblemID uuid.UUID
	ContestID *uuid.UUID
	After     *SubmissionCursor
	Limit     int
}

func (query MySubmissionQuery) Validate() error {
	if query.UserID == uuid.Nil || query.ProblemID == uuid.Nil ||
		(query.ContestID != nil && *query.ContestID == uuid.Nil) ||
		query.Limit < 1 || query.Limit > MaximumMySubmissionPageLimit {
		return ErrInvalidMySubmissionPage
	}
	if query.After != nil &&
		(query.After.ID == uuid.Nil || query.After.CreatedAt.IsZero() ||
			!query.After.CreatedAt.After(time.Unix(0, 0))) {
		return ErrInvalidMySubmissionPage
	}
	return nil
}

type MySubmissionPage struct {
	Submissions []*entity.Submission
	Next        *SubmissionCursor
}

// SubmissionRepository defines the submission data access interface.
type SubmissionRepository interface {
	Get(ctx context.Context, id uuid.UUID) (*entity.Submission, error)
	GetOwned(ctx context.Context, id, userID uuid.UUID) (*entity.Submission, error)
	// Summary queries return the newest bounded window in deterministic order.
	// They must never load an unbounded submission history into API memory.
	FindByProblemID(ctx context.Context, problemID uuid.UUID) ([]*entity.Submission, error)
	FindMyByProblem(ctx context.Context, query MySubmissionQuery) (MySubmissionPage, error)
	FindAdmin(ctx context.Context, query entity.AdminSubmissionQuery) (*commondto.Paginated[*entity.Submission], error)
}

// SubmissionService defines the submission business logic interface.
type SubmissionService interface {
	Get(ctx context.Context, userID, id uuid.UUID) (*contract.SubmissionResponse, error)
	FindByProblemID(ctx context.Context, problemID uuid.UUID) ([]*contract.SubmissionSummaryResponse, error)
	FindMyByProblem(ctx context.Context, userID uuid.UUID, req *contract.MySubmissionsRequest) (*contract.MySubmissionsPageResponse, error)
	FindAdmin(ctx context.Context, req *contract.AdminSubmissionFindRequest) (*commondto.Paginated[*contract.AdminSubmissionSummaryResponse], error)
	GetAdmin(ctx context.Context, id uuid.UUID) (*contract.AdminSubmissionDetailResponse, error)
}

// SubmissionCommandService isolates the durable write workflow from bounded
// public/admin projections and keeps application orchestration out of HTTP.
type SubmissionCommandService interface {
	Submit(
		ctx context.Context,
		userID uuid.UUID,
		idempotencyKey string,
		command *contract.CreateSubmissionCommand,
	) (*contract.SubmissionAcceptedResponse, error)
}

// OperationActor is the authenticated administrative principal recorded in
// the immutable audit trail. Source authorization remains at the HTTP policy
// boundary; this type exists only to preserve durable accountability.
type OperationActor struct {
	UserID    uuid.UUID
	SessionID *uuid.UUID
}

// RejudgeCommand asks for one next immutable generation. The idempotency key
// is deliberately a UUID, distinct from submission intake's opaque header,
// so the database can derive stable command identities without a mutable
// receipt table.
type RejudgeCommand struct {
	SubmissionID    uuid.UUID
	ExpectedVersion int64
	Reason          string
	IdempotencyKey  uuid.UUID
}

// CancelCommand closes exactly the observed generation. A cancellation never
// erases a previously active contestant judgement.
type CancelCommand struct {
	SubmissionID    uuid.UUID
	Generation      int64
	ExpectedVersion int64
	Reason          string
}

// OperationResult is intentionally browser-safe: it reveals durable state,
// not source, leases, hidden cases, or worker topology.
type OperationResult struct {
	SubmissionID     uuid.UUID
	Generation       int64
	AggregateVersion int64
	Phase            entity.Phase
	Outcome          string
	JobID            *uuid.UUID
	IdempotentReplay bool
}

// SubmissionOperationsRepository owns the one-transaction durable mutation
// boundary for administrator judge operations.
type SubmissionOperationsRepository interface {
	Rejudge(context.Context, OperationActor, RejudgeCommand) (OperationResult, error)
	Cancel(context.Context, OperationActor, CancelCommand) (OperationResult, error)
}

// SubmissionOperations exposes explicit operational mutations separately
// from ordinary contestant submissions and browser read projections.
type SubmissionOperations interface {
	Rejudge(context.Context, OperationActor, RejudgeCommand) (OperationResult, error)
	Cancel(context.Context, OperationActor, CancelCommand) (OperationResult, error)
}
