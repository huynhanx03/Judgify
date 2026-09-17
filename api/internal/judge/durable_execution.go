package judge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/judge/checker"
	"github.com/huynhanx03/judgify/internal/judge/executor"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	submissionports "github.com/huynhanx03/judgify/internal/ports"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	problementity "github.com/huynhanx03/judgify/internal/problem/domain"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
)

const maximumAttemptWallTime = 24 * time.Hour

var ErrLeaseExecutorInvalid = errors.New("durable lease executor: invalid operation")

type LeaseJobCoordinator interface {
	TransitionPhase(
		context.Context,
		submissionports.JobLease,
		int64,
		submissionentity.Phase,
	) (submissionentity.SubmissionState, error)
	RecordRetry(
		context.Context,
		submissionports.JobLease,
		submissionports.InfrastructureIncident,
	) error
}

type LeaseCompiler interface {
	Compile(
		context.Context,
		executor.CompileRequest,
	) (executor.CompileOutcome, error)
}

type LeaseTestcaseRunner interface {
	Run(
		context.Context,
		executor.TestcaseRequest,
	) (executor.CaseResult, error)
}

type LeaseAssetDecoder interface {
	Decode(
		context.Context,
		submissionports.JudgeAssetSnapshot,
	) ([]byte, error)
}

type LeaseContestantFinalizer interface {
	FinalizeContestantEvaluation(
		context.Context,
		submissionports.FinalizationRequest,
	) (submissionports.FinalizationResult, error)
}

type CompiledArtifactReleaser interface {
	Release(context.Context, *executor.CompiledArtifact) error
}

// StagedArtifactReleaser adapts the executor's private staging lease to the
// application-level worker pipeline without exposing staging paths.
type StagedArtifactReleaser struct{}

func (StagedArtifactReleaser) Release(
	ctx context.Context,
	artifact *executor.CompiledArtifact,
) error {
	if artifact == nil {
		return executor.ErrInvalidArtifact
	}
	return artifact.Release(ctx)
}

type LeaseExecutorDependencies struct {
	Plans     submissionports.JudgeSpecReader
	Cases     submissionports.JudgeCaseReader
	Catalog   executor.RuntimeProfileCatalog
	Jobs      LeaseJobCoordinator
	Compiler  LeaseCompiler
	Testcases LeaseTestcaseRunner
	Assets    LeaseAssetDecoder
	Finalizer LeaseContestantFinalizer
	Artifacts CompiledArtifactReleaser
}

type LeaseExecutorConfig struct {
	MaximumSourceBytes int64
	FileBytes          int64
	TmpfsBytes         int64
	DiagnosticBytes    int64
	OpenFiles          int64
	CleanupTimeout     time.Duration
}

// LeaseExecutor is the one production path from a fenced PostgreSQL lease to
// either an atomic contestant finalization or a durable infrastructure retry.
type LeaseExecutor struct {
	dependencies LeaseExecutorDependencies
	config       LeaseExecutorConfig
	now          func() time.Time
	newID        func() (uuid.UUID, error)
}

func NewDurableLeaseExecutor(
	dependencies LeaseExecutorDependencies,
	config LeaseExecutorConfig,
) (*LeaseExecutor, error) {
	if nilLeaseExecutionDependency(dependencies.Plans) ||
		nilLeaseExecutionDependency(dependencies.Cases) ||
		nilLeaseExecutionDependency(dependencies.Catalog) ||
		nilLeaseExecutionDependency(dependencies.Jobs) ||
		nilLeaseExecutionDependency(dependencies.Compiler) ||
		nilLeaseExecutionDependency(dependencies.Testcases) ||
		nilLeaseExecutionDependency(dependencies.Assets) ||
		nilLeaseExecutionDependency(dependencies.Finalizer) ||
		nilLeaseExecutionDependency(dependencies.Artifacts) ||
		config.MaximumSourceBytes < 1 ||
		config.MaximumSourceBytes > judging.MaximumSourceCodeBytes ||
		config.FileBytes < config.MaximumSourceBytes ||
		config.FileBytes > 256<<20 ||
		config.TmpfsBytes < config.FileBytes ||
		config.TmpfsBytes > 512<<20 ||
		config.DiagnosticBytes < 1 ||
		config.DiagnosticBytes > 1<<20 ||
		config.OpenFiles < 16 ||
		config.OpenFiles > 1024 ||
		config.CleanupTimeout < time.Second ||
		config.CleanupTimeout > time.Minute {
		return nil, ErrLeaseExecutorInvalid
	}
	return &LeaseExecutor{
		dependencies: dependencies,
		config:       config,
		now:          func() time.Time { return time.Now().UTC() },
		newID:        uuid.NewV7,
	}, nil
}

func (worker *LeaseExecutor) ExecuteLease(
	parent context.Context,
	lease submissionports.JobLease,
) error {
	if worker == nil || parent == nil || parent.Err() != nil ||
		validateExecutionLease(lease) != nil {
		return ErrLeaseExecutorInvalid
	}
	ctx := leaseLogContext(parent, lease)
	log := logger.FromContext(ctx)
	log.Debug("durable judge attempt preparation started")

	plan, err := worker.dependencies.Plans.ReadJudgePlan(ctx, lease)
	if err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "judge_plan_read_failed",
				Summary:        "the immutable judge plan could not be verified",
			},
			err,
		)
	}
	if err := validateLeaseExecutionPlan(plan, lease); err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "judge_plan_integrity_failed",
				Summary:        "the immutable judge plan failed worker validation",
			},
			err,
		)
	}
	profile, err := executor.ResolveRuntimeProfile(worker.dependencies.Catalog, plan)
	if err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "capability",
				Code:           "runtime_profile_mismatch",
				Summary:        "the claimed runtime profile did not match the worker catalog",
			},
			err,
		)
	}
	implementationChecksum, implementationKnown := checker.ImplementationChecksum(
		checker.Identity{Key: plan.Checker.Key, Version: plan.Checker.Version},
	)
	if !implementationKnown ||
		implementationChecksum != plan.Release.CheckerImplementationChecksum ||
		implementationChecksum != plan.Checker.ImplementationChecksum {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "checker_implementation_mismatch",
				Summary:        "the trusted checker implementation did not match the released plan",
			},
			ErrUntrustedExecution,
		)
	}
	builtinChecker, baseCheckerPolicy, err := checker.Configure(
		checker.Identity{Key: plan.Checker.Key, Version: plan.Checker.Version},
		plan.Checker.Config,
		problemcontract.MaximumTestAssetBytes,
		int64(plan.Limits.OutputBytes),
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "capability",
				Code:           "checker_profile_mismatch",
				Summary:        "the claimed checker profile did not match the worker implementation",
			},
			err,
		)
	}

	attemptCtx, cancelAttempt := context.WithTimeout(
		ctx,
		time.Duration(plan.Limits.TotalAttemptWallMilliseconds)*time.Millisecond,
	)
	defer cancelAttempt()

	compiling, err := worker.transitionPhase(
		attemptCtx,
		lease,
		lease.SubmissionVersion,
		submissionentity.PhaseCompiling,
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			phaseTransitionIncident("compiling"),
			err,
		)
	}

	source := []byte(plan.SourceCode)
	plan.SourceCode = ""
	compileOutcome, err := worker.dependencies.Compiler.Compile(
		attemptCtx,
		executor.CompileRequest{
			Identity:           executionIdentity(lease),
			RuntimeProfile:     profile,
			Source:             source,
			SourceChecksum:     digestText(plan.SourceSHA256),
			MaximumSourceBytes: worker.config.MaximumSourceBytes,
			Limits:             worker.compileLimits(plan),
		},
	)
	clear(source)
	if err != nil {
		clearCompileOutcome(&compileOutcome)
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "compile_boundary_failed",
				Summary:        "the disposable compilation boundary failed",
			},
			err,
		)
	}
	if incident, validationErr := validateCompileOutcome(
		compileOutcome,
		lease,
		profile,
	); validationErr != nil {
		clearCompileOutcome(&compileOutcome)
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			incident,
			validationErr,
		)
	}
	switch compileOutcome.Status {
	case executor.CompileStatusRejected, executor.CompileStatusLimitExceeded:
		diagnostic := safeCompileDiagnostic(
			compileOutcome.Status,
			compileOutcome.Diagnostic,
			submissionentity.MaximumJudgeDiagnosticBytes,
		)
		clearCompileOutcome(&compileOutcome)
		finalizing, transitionErr := worker.transitionPhase(
			attemptCtx,
			lease,
			compiling.AggregateVersion,
			submissionentity.PhaseFinalizing,
		)
		if transitionErr != nil {
			return worker.handleInfrastructureFailure(
				attemptCtx,
				parent,
				lease,
				phaseTransitionIncident("finalizing"),
				transitionErr,
			)
		}
		return worker.finalizeContestant(
			attemptCtx,
			parent,
			lease,
			plan,
			finalizing.AggregateVersion,
			submissionentity.VerdictCompileError,
			nil,
			diagnostic,
		)
	case executor.CompileStatusSucceeded:
		if compileOutcome.Artifact == nil {
			clearCompileOutcome(&compileOutcome)
			return worker.handleInfrastructureFailure(
				attemptCtx,
				parent,
				lease,
				submissionports.InfrastructureIncident{
					Classification: "integrity",
					Code:           "compiled_artifact_missing",
					Summary:        "successful compilation produced no staged artifact",
				},
				executor.ErrCompileInfrastructure,
			)
		}
	default:
		clearCompileOutcome(&compileOutcome)
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "compile_status_invalid",
				Summary:        "the compiler returned an unknown status",
			},
			ErrLeaseExecutorInvalid,
		)
	}

	artifact := compileOutcome.Artifact
	compileOutcome.Artifact = nil
	clearCompileOutcome(&compileOutcome)
	artifactOwned := true
	defer func() {
		if !artifactOwned {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			worker.config.CleanupTimeout,
		)
		defer cancel()
		if releaseErr := worker.dependencies.Artifacts.Release(cleanupCtx, artifact); releaseErr != nil {
			log.Error("staged compiled artifact fallback cleanup failed", zap.Error(releaseErr))
		}
	}()

	running, err := worker.transitionPhase(
		attemptCtx,
		lease,
		compiling.AggregateVersion,
		submissionentity.PhaseRunning,
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			phaseTransitionIncident("running"),
			err,
		)
	}
	evaluation, err := worker.executeCases(
		attemptCtx,
		lease,
		plan,
		artifact,
		builtinChecker,
		baseCheckerPolicy,
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			evaluation.incident,
			err,
		)
	}
	if err := worker.releaseArtifact(ctx, artifact); err != nil {
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "artifact_cleanup_failed",
				Summary:        "the staged compiled artifact could not be removed",
			},
			err,
		)
	}
	artifactOwned = false

	finalizing, err := worker.transitionPhase(
		attemptCtx,
		lease,
		running.AggregateVersion,
		submissionentity.PhaseFinalizing,
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			attemptCtx,
			parent,
			lease,
			phaseTransitionIncident("finalizing"),
			err,
		)
	}
	return worker.finalizeContestant(
		attemptCtx,
		parent,
		lease,
		plan,
		finalizing.AggregateVersion,
		evaluation.verdict,
		evaluation.results,
		nil,
	)
}

type caseEvaluation struct {
	verdict  submissionentity.Verdict
	results  []submissionports.JudgeTestResult
	incident submissionports.InfrastructureIncident
}

func (worker *LeaseExecutor) executeCases(
	ctx context.Context,
	lease submissionports.JobLease,
	plan submissionports.JudgePlan,
	artifact *executor.CompiledArtifact,
	builtinChecker checker.Checker,
	basePolicy checker.Policy,
) (caseEvaluation, error) {
	evaluation := caseEvaluation{
		verdict: submissionentity.VerdictAccepted,
		results: make([]submissionports.JudgeTestResult, 0, len(plan.Tests)),
	}
	skipGroup := -1
	for _, test := range plan.Tests {
		if skipGroup == test.GroupIndex {
			continue
		}
		if skipGroup >= 0 && skipGroup != test.GroupIndex {
			skipGroup = -1
		}
		payload, err := worker.dependencies.Cases.ReadJudgeCase(ctx, lease, test.RevisionID)
		if err != nil {
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_asset_read_failed",
				Summary:        "an immutable testcase asset could not be read",
			}
			return evaluation, err
		}
		if !judgeCaseMatchesSpec(payload, test) {
			clearJudgeCasePayload(&payload)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_asset_mismatch",
				Summary:        "a testcase asset did not match the immutable judge plan",
			}
			return evaluation, submissionports.ErrJudgeCaseIntegrity
		}
		input, err := worker.dependencies.Assets.Decode(ctx, payload.Input)
		clear(payload.Input.Content)
		if err != nil {
			clear(payload.Expected.Content)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_input_decode_failed",
				Summary:        "a testcase input failed integrity decoding",
			}
			return evaluation, err
		}
		expected, err := worker.dependencies.Assets.Decode(ctx, payload.Expected)
		clear(payload.Expected.Content)
		if err != nil {
			clear(input)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_expected_decode_failed",
				Summary:        "a testcase expected output failed integrity decoding",
			}
			return evaluation, err
		}

		startedAt := worker.now()
		caseResult, err := worker.dependencies.Testcases.Run(
			ctx,
			executor.TestcaseRequest{
				Identity:   executionIdentity(lease),
				TestcaseID: test.RevisionID,
				Artifact:   artifact,
				Input:      input,
				Limits:     worker.testcaseLimits(plan),
			},
		)
		clear(input)
		endedAt := worker.now()
		if err != nil {
			clear(expected)
			clearCaseResult(&caseResult)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "testcase_boundary_failed",
				Summary:        "a disposable testcase boundary failed",
			}
			return evaluation, err
		}
		if caseResult.TestcaseID != test.RevisionID {
			clear(expected)
			clearCaseResult(&caseResult)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_result_mismatch",
				Summary:        "a testcase result did not match its execution request",
			}
			return evaluation, ErrUntrustedExecution
		}

		checked := checker.Result{}
		if executionNeedsChecker(caseResult.Execution) {
			policy := basePolicy
			policy.MaximumExpectedBytes = maxInt64(1, test.OutputBytes)
			checked, err = builtinChecker.Check(
				ctx,
				bytes.NewReader(expected),
				bytes.NewReader(caseResult.Output),
				policy,
			)
			if err != nil {
				clear(expected)
				clearCaseResult(&caseResult)
				evaluation.incident = submissionports.InfrastructureIncident{
					Classification: "integrity",
					Code:           "trusted_checker_failed",
					Summary:        "the trusted checker could not evaluate testcase output",
				}
				return evaluation, err
			}
		}
		clear(expected)
		verdict, err := Classify(caseResult.Execution, checked)
		if err != nil {
			clearCaseResult(&caseResult)
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "execution_evidence_invalid",
				Summary:        "sandbox execution evidence failed trusted validation",
			}
			return evaluation, err
		}
		result, err := buildJudgeTestResult(
			plan,
			test,
			len(evaluation.results),
			verdict,
			checked,
			caseResult,
			startedAt,
			endedAt,
		)
		clearCaseResult(&caseResult)
		if err != nil {
			evaluation.incident = submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "testcase_result_invalid",
				Summary:        "structured testcase evidence failed validation",
			}
			return evaluation, err
		}
		evaluation.results = append(evaluation.results, result)
		if verdict != submissionentity.VerdictAccepted {
			if evaluation.verdict == submissionentity.VerdictAccepted {
				evaluation.verdict = verdict
			}
			if test.StopOnFailure {
				skipGroup = test.GroupIndex
			}
		}
	}
	if len(evaluation.results) == 0 {
		evaluation.incident = submissionports.InfrastructureIncident{
			Classification: "integrity",
			Code:           "testcase_execution_empty",
			Summary:        "the judge plan produced no testcase execution",
		}
		return evaluation, ErrLeaseExecutorInvalid
	}
	return evaluation, nil
}

func (worker *LeaseExecutor) finalizeContestant(
	ctx context.Context,
	parent context.Context,
	lease submissionports.JobLease,
	plan submissionports.JudgePlan,
	expectedVersion int64,
	verdict submissionentity.Verdict,
	results []submissionports.JudgeTestResult,
	compileDiagnostic *string,
) error {
	judgementID, err := worker.newID()
	if err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "judgement_identity_failed",
				Summary:        "a durable judgement identity could not be created",
			},
			err,
		)
	}
	passed, maximumTime, maximumMemory := aggregateTestResults(results)
	attemptID := lease.Attempt.ID
	judgement := submissionentity.Judgement{
		ID:                              judgementID,
		SubmissionID:                    plan.SubmissionID,
		Generation:                      plan.Generation,
		JudgeSpecID:                     plan.JudgeSpecID,
		ProblemRevisionID:               plan.ProblemRevisionID,
		TestsetRevisionID:               plan.TestsetRevisionID,
		Verdict:                         verdict,
		PassedCount:                     passed,
		TotalCount:                      len(plan.Tests),
		MaximumTimeMilliseconds:         maximumTime,
		MaximumMemoryKilobytes:          maximumMemory,
		CompileDiagnosticSummary:        compileDiagnostic,
		JudgeSpecChecksum:               plan.SpecChecksum,
		RewardSnapshot:                  append(json.RawMessage(nil), plan.RewardFacts...),
		RewardSnapshotChecksum:          plan.RewardChecksum,
		RuntimeProfileVersion:           plan.Runtime.ProfileVersion,
		CompilerVersion:                 plan.Runtime.CompilerVersion,
		RuntimeBootstrapVersion:         plan.Runtime.BootstrapVersion,
		SupervisorProtocolVersion:       plan.Runtime.SupervisorProtocol,
		SandboxImageDigest:              plan.Runtime.SandboxImageDigest,
		ArtifactManifestChecksum:        plan.Runtime.ArtifactManifestChecksum,
		ReleaseID:                       plan.Release.ID,
		ApprovedReleaseManifestChecksum: plan.Release.ApprovedManifestChecksum,
		CheckerImplementationChecksum:   plan.Release.CheckerImplementationChecksum,
		WinningAttemptID:                &attemptID,
		CompletedAt:                     worker.now(),
		OriginCID:                       plan.OriginCID,
	}
	_, err = worker.dependencies.Finalizer.FinalizeContestantEvaluation(
		ctx,
		submissionports.FinalizationRequest{
			Lease:                     lease,
			ExpectedSubmissionVersion: expectedVersion,
			Judgement:                 judgement,
			TestResults:               results,
		},
	)
	if err != nil {
		return worker.handleInfrastructureFailure(
			ctx,
			parent,
			lease,
			submissionports.InfrastructureIncident{
				Classification: "infrastructure",
				Code:           "judgement_finalization_failed",
				Summary:        "the atomic judgement finalization did not commit",
			},
			err,
		)
	}
	logger.FromContext(ctx).Info(
		"durable judge attempt finalized",
		zap.String("verdict", string(verdict)),
		zap.Int("passed_count", passed),
		zap.Int("total_count", len(plan.Tests)),
	)
	return nil
}

func (worker *LeaseExecutor) handleInfrastructureFailure(
	ctx context.Context,
	parent context.Context,
	lease submissionports.JobLease,
	incident submissionports.InfrastructureIncident,
	cause error,
) error {
	if parent.Err() != nil {
		return parent.Err()
	}
	if errors.Is(cause, submissionports.ErrLeaseLost) ||
		errors.Is(cause, submissionports.ErrLeaseCancelled) ||
		errors.Is(cause, submissionports.ErrWorkerBootLost) ||
		errors.Is(cause, submissionports.ErrFinalizationConflict) {
		return cause
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		incident = submissionports.InfrastructureIncident{
			Classification: "infrastructure",
			Code:           "attempt_deadline_exceeded",
			Summary:        "the total judge attempt deadline was exceeded",
		}
	}
	log := logger.FromContext(ctx).With(
		zap.String("failure_classification", incident.Classification),
		zap.String("failure_code", incident.Code),
	)
	// Compiler and sandbox failures can wrap contestant-controlled diagnostics.
	// Keep the stable incident classification and concrete Go error type, never
	// the raw error string, source excerpt, command output, or sandbox payload.
	log.Warn(
		"durable judge attempt entering retry policy",
		zap.String("cause_type", fmt.Sprintf("%T", cause)),
	)
	retryContext := leaseLogContext(parent, lease)
	if err := worker.dependencies.Jobs.RecordRetry(retryContext, lease, incident); err != nil {
		log.Error("durable judge retry recording failed", zap.Error(err))
		return errors.Join(cause, err)
	}
	return nil
}

func (worker *LeaseExecutor) transitionPhase(
	ctx context.Context,
	lease submissionports.JobLease,
	expectedVersion int64,
	phase submissionentity.Phase,
) (submissionentity.SubmissionState, error) {
	state, err := worker.dependencies.Jobs.TransitionPhase(
		ctx,
		lease,
		expectedVersion,
		phase,
	)
	if err != nil {
		return submissionentity.SubmissionState{}, err
	}
	if state.ID != lease.Job.SubmissionID ||
		state.LatestGeneration != lease.Job.Generation ||
		state.LatestPhase != phase ||
		state.AggregateVersion != expectedVersion+1 ||
		state.LatestOutcome != "" {
		return submissionentity.SubmissionState{}, ErrLeaseExecutorInvalid
	}
	return state, nil
}

func (worker *LeaseExecutor) releaseArtifact(
	ctx context.Context,
	artifact *executor.CompiledArtifact,
) error {
	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		worker.config.CleanupTimeout,
	)
	defer cancel()
	return worker.dependencies.Artifacts.Release(cleanupCtx, artifact)
}

func (worker *LeaseExecutor) compileLimits(
	plan submissionports.JudgePlan,
) supervisor.ResourceLimits {
	return supervisor.ResourceLimits{
		WallTime:        time.Duration(plan.Limits.CompileWallTimeMilliseconds) * time.Millisecond,
		CPUTime:         time.Duration(plan.Limits.CompileCPUTimeMilliseconds) * time.Millisecond,
		MemoryBytes:     int64(plan.Limits.MemoryKilobytes) << 10,
		Pids:            int64(plan.Limits.ProcessCount),
		OutputBytes:     int64(plan.Limits.OutputBytes),
		DiagnosticBytes: worker.config.DiagnosticBytes,
		FileBytes:       worker.config.FileBytes,
		TmpfsBytes:      worker.config.TmpfsBytes,
		OpenFiles:       worker.config.OpenFiles,
	}
}

func (worker *LeaseExecutor) testcaseLimits(
	plan submissionports.JudgePlan,
) supervisor.ResourceLimits {
	return supervisor.ResourceLimits{
		WallTime:        time.Duration(plan.Limits.WallTimeMilliseconds) * time.Millisecond,
		CPUTime:         time.Duration(plan.Limits.CPUTimeMilliseconds) * time.Millisecond,
		MemoryBytes:     int64(plan.Limits.MemoryKilobytes) << 10,
		Pids:            int64(plan.Limits.ProcessCount),
		OutputBytes:     int64(plan.Limits.OutputBytes),
		DiagnosticBytes: worker.config.DiagnosticBytes,
		FileBytes:       worker.config.FileBytes,
		TmpfsBytes:      worker.config.TmpfsBytes,
		OpenFiles:       worker.config.OpenFiles,
	}
}

func validateExecutionLease(lease submissionports.JobLease) error {
	if lease.Job.ID == uuid.Nil ||
		lease.Job.SubmissionID == uuid.Nil ||
		lease.Job.JudgeSpecID == uuid.Nil ||
		lease.Job.Generation < 1 ||
		lease.Job.OriginCID == "" ||
		lease.Attempt.ID == uuid.Nil ||
		lease.Attempt.JobID != lease.Job.ID ||
		lease.Attempt.SubmissionID != lease.Job.SubmissionID ||
		lease.Attempt.Generation != lease.Job.Generation ||
		lease.Attempt.JudgeSpecID != lease.Job.JudgeSpecID ||
		lease.Attempt.AttemptNumber < 1 ||
		lease.Attempt.WorkerID == uuid.Nil ||
		lease.Attempt.WorkerBootID == uuid.Nil ||
		lease.JudgeSpec.ID != lease.Job.JudgeSpecID ||
		lease.JudgeSpec.SubmissionID != lease.Job.SubmissionID ||
		lease.JudgeSpec.Generation != lease.Job.Generation ||
		lease.JudgeSpec.ReleaseID == "" ||
		lease.JudgeSpec.ApprovedManifestChecksum == "" ||
		lease.JudgeSpec.CheckerImplementationChecksum == "" ||
		lease.LeaseOwner == "" ||
		len(lease.LeaseToken) != sha256.Size ||
		lease.SubmissionVersion < 1 ||
		correlation.Validate(lease.Job.OriginCID) != nil {
		return ErrLeaseExecutorInvalid
	}
	return nil
}

func validateLeaseExecutionPlan(
	plan submissionports.JudgePlan,
	lease submissionports.JobLease,
) error {
	if plan.JobID != lease.Job.ID ||
		plan.SubmissionID != lease.Job.SubmissionID ||
		plan.JudgeSpecID != lease.Job.JudgeSpecID ||
		plan.Generation != lease.Job.Generation ||
		plan.UserID != lease.JudgeSpec.UserID ||
		plan.ProblemID == uuid.Nil ||
		plan.ProblemRevisionID != lease.JudgeSpec.ProblemRevisionID ||
		plan.TestsetRevisionID != lease.JudgeSpec.TestsetRevisionID ||
		plan.SourceSHA256 != sha256.Sum256([]byte(plan.SourceCode)) ||
		judging.ValidateSourceCode(plan.SourceCode) != nil ||
		plan.SpecChecksum == ([32]byte{}) ||
		plan.RewardChecksum == ([32]byte{}) ||
		plan.TestsetChecksum == ([32]byte{}) ||
		plan.OriginCID != lease.Job.OriginCID ||
		len(plan.RewardFacts) == 0 ||
		len(plan.RewardFacts) > problemcontract.MaximumRewardSnapshotJSONBytes ||
		!json.Valid(plan.RewardFacts) ||
		len(plan.Tests) < 1 ||
		len(plan.Tests) > problemcontract.MaximumTestCases ||
		!validJudgeExecutionLimits(plan.Limits, len(plan.Tests)) {
		return submissionports.ErrJudgePlanIntegrity
	}
	var rewardObject map[string]json.RawMessage
	if json.Unmarshal(plan.RewardFacts, &rewardObject) != nil || rewardObject == nil {
		return submissionports.ErrJudgePlanIntegrity
	}
	seenRevisions := make(map[uuid.UUID]struct{}, len(plan.Tests))
	seenStableCases := make(map[uuid.UUID]struct{}, len(plan.Tests))
	groupIDs := make(map[int]uuid.UUID)
	groupStop := make(map[int]bool)
	expectedGroup := 0
	expectedCase := 0
	for index, test := range plan.Tests {
		if index == 0 {
			expectedGroup = test.GroupIndex
		} else if test.GroupIndex != expectedGroup {
			if test.GroupIndex != expectedGroup+1 {
				return submissionports.ErrJudgePlanIntegrity
			}
			expectedGroup = test.GroupIndex
			expectedCase = 0
		}
		if test.RevisionID == uuid.Nil ||
			test.TestCaseID == uuid.Nil ||
			test.GroupID == uuid.Nil ||
			test.GroupIndex < 0 ||
			test.GroupIndex >= problemcontract.MaximumTestcaseGroups ||
			test.OrderIndex != expectedCase ||
			!problemcontract.IsTestKind(test.Kind) ||
			test.InputAssetID == uuid.Nil ||
			test.OutputAssetID == uuid.Nil ||
			test.InputSHA256 == ([32]byte{}) ||
			test.OutputSHA256 == ([32]byte{}) ||
			!problemcontract.IsAssetEncoding(test.InputEncoding) ||
			!problemcontract.IsAssetEncoding(test.OutputEncoding) ||
			test.InputBytes < 0 ||
			test.InputBytes > problemcontract.MaximumTestAssetBytes ||
			test.OutputBytes < 0 ||
			test.OutputBytes > problemcontract.MaximumTestAssetBytes {
			return submissionports.ErrJudgePlanIntegrity
		}
		if groupID, exists := groupIDs[test.GroupIndex]; exists {
			if groupID != test.GroupID || groupStop[test.GroupIndex] != test.StopOnFailure {
				return submissionports.ErrJudgePlanIntegrity
			}
		} else {
			groupIDs[test.GroupIndex] = test.GroupID
			groupStop[test.GroupIndex] = test.StopOnFailure
		}
		if _, duplicate := seenRevisions[test.RevisionID]; duplicate {
			return submissionports.ErrJudgePlanIntegrity
		}
		if _, duplicate := seenStableCases[test.TestCaseID]; duplicate {
			return submissionports.ErrJudgePlanIntegrity
		}
		seenRevisions[test.RevisionID] = struct{}{}
		seenStableCases[test.TestCaseID] = struct{}{}
		expectedCase++
	}
	if plan.Tests[0].GroupIndex != 0 {
		return submissionports.ErrJudgePlanIntegrity
	}
	return nil
}

func validJudgeExecutionLimits(
	limits submissionports.JudgeLimits,
	testCount int,
) bool {
	if testCount < 1 ||
		limits.TotalAttemptWallMilliseconds < 1 ||
		int64(limits.TotalAttemptWallMilliseconds) >
			int64(maximumAttemptWallTime/time.Millisecond) {
		return false
	}
	expectedTotal := int64(limits.CompileWallTimeMilliseconds) +
		int64(limits.WallTimeMilliseconds)*int64(testCount)
	if expectedTotal < int64(limits.CompileWallTimeMilliseconds) ||
		expectedTotal > int64(maximumAttemptWallTime/time.Millisecond) ||
		int64(limits.TotalAttemptWallMilliseconds) != expectedTotal {
		return false
	}
	total := time.Duration(limits.TotalAttemptWallMilliseconds) * time.Millisecond
	return limits.CPUTimeMilliseconds >= problemcontract.MinimumCPUTimeLimitMilliseconds &&
		limits.CPUTimeMilliseconds <= problemcontract.MaximumCPUTimeLimitMilliseconds &&
		limits.WallTimeMilliseconds >= limits.CPUTimeMilliseconds &&
		limits.WallTimeMilliseconds <= problemcontract.MaximumWallTimeLimitMilliseconds &&
		limits.MemoryKilobytes >= problemcontract.MinimumMemoryLimitKilobytes &&
		limits.MemoryKilobytes <= problemcontract.MaximumMemoryLimitKilobytes &&
		limits.OutputBytes >= problemcontract.MinimumOutputLimitBytes &&
		limits.OutputBytes <= problemcontract.MaximumOutputLimitBytes &&
		limits.ProcessCount >= problemcontract.MinimumProcessLimit &&
		limits.ProcessCount <= problemcontract.MaximumProcessLimit &&
		limits.CompileCPUTimeMilliseconds >= problemcontract.MinimumCPUTimeLimitMilliseconds &&
		limits.CompileCPUTimeMilliseconds <= problemcontract.MaximumWallTimeLimitMilliseconds &&
		limits.CompileWallTimeMilliseconds >= limits.CompileCPUTimeMilliseconds &&
		limits.CompileWallTimeMilliseconds <= problemcontract.MaximumWallTimeLimitMilliseconds &&
		limits.TotalAttemptWallMilliseconds >= limits.CompileWallTimeMilliseconds &&
		total <= maximumAttemptWallTime
}

func judgeCaseMatchesSpec(
	payload submissionports.JudgeCasePayload,
	spec submissionports.JudgeTestSpec,
) bool {
	return payload.TestcaseRevisionID == spec.RevisionID &&
		payload.Input.ID == spec.InputAssetID &&
		payload.Input.SHA256 == spec.InputSHA256 &&
		payload.Input.Encoding == spec.InputEncoding &&
		payload.Input.UncompressedSize == spec.InputBytes &&
		payload.Expected.ID == spec.OutputAssetID &&
		payload.Expected.SHA256 == spec.OutputSHA256 &&
		payload.Expected.Encoding == spec.OutputEncoding &&
		payload.Expected.UncompressedSize == spec.OutputBytes
}

func buildJudgeTestResult(
	plan submissionports.JudgePlan,
	test submissionports.JudgeTestSpec,
	executionOrder int,
	verdict submissionentity.Verdict,
	checked checker.Result,
	result executor.CaseResult,
	startedAt time.Time,
	endedAt time.Time,
) (submissionports.JudgeTestResult, error) {
	if startedAt.IsZero() || endedAt.Before(startedAt) {
		return submissionports.JudgeTestResult{}, ErrLeaseExecutorInvalid
	}
	checkerCode := checked.Code
	if checkerCode == "" {
		checkerCode = "not_run"
	}
	metadata, err := json.Marshal(struct {
		CheckerKey     string `json:"checker_key"`
		CheckerVersion string `json:"checker_version"`
		Code           string `json:"code"`
	}{
		CheckerKey:     plan.Checker.Key,
		CheckerVersion: plan.Checker.Version,
		Code:           checkerCode,
	})
	if err != nil {
		return submissionports.JudgeTestResult{}, err
	}
	exitCode := result.Execution.ExitCode
	return submissionports.JudgeTestResult{
		JudgeSpecID:        plan.JudgeSpecID,
		TestsetRevisionID:  plan.TestsetRevisionID,
		TestcaseRevisionID: test.RevisionID,
		GroupID:            test.GroupID,
		GroupIndex:         test.GroupIndex,
		CaseOrder:          test.OrderIndex,
		ExecutionOrder:     executionOrder,
		Kind:               test.Kind,
		StopOnFailure:      test.StopOnFailure,
		Verdict:            verdict,
		TimeMilliseconds:   boundedMilliseconds(result.Execution.WallTime),
		MemoryKilobytes:    boundedKilobytes(result.Execution.PeakMemoryBytes),
		ExitCode:           &exitCode,
		StdoutBytes:        len(result.Output),
		StderrBytes:        len(result.Execution.Stderr),
		CheckerMetadata:    metadata,
		ExecutionStartedAt: startedAt.UTC(),
		ExecutionEndedAt:   endedAt.UTC(),
	}, nil
}

func executionNeedsChecker(result supervisor.ExecuteResult) bool {
	return !result.OOMKilled &&
		!result.MemoryLimitExceeded &&
		!result.ProcessLimitExceeded &&
		!result.OutputLimitExceeded &&
		!result.CPUTimeLimitExceeded &&
		!result.WallTimeLimitExceeded &&
		result.ExitCode == 0 &&
		result.Signal == ""
}

func aggregateTestResults(
	results []submissionports.JudgeTestResult,
) (int, *int, *int) {
	if len(results) == 0 {
		return 0, nil, nil
	}
	passed := 0
	maximumTime := 0
	maximumMemory := 0
	for _, result := range results {
		if result.Verdict == submissionentity.VerdictAccepted {
			passed++
		}
		maximumTime = max(maximumTime, result.TimeMilliseconds)
		maximumMemory = max(maximumMemory, result.MemoryKilobytes)
	}
	return passed, &maximumTime, &maximumMemory
}

func safeCompileDiagnostic(
	status executor.CompileStatus,
	value []byte,
	maximumBytes int,
) *string {
	if status == executor.CompileStatusLimitExceeded {
		text := "Compilation exceeded the configured resource limit."
		return &text
	}
	if maximumBytes < 1 || len(value) == 0 {
		return nil
	}
	normalized := strings.ToValidUTF8(string(value), "\uFFFD")
	var result strings.Builder
	result.Grow(min(len(normalized), maximumBytes))
	for _, character := range normalized {
		if character != '\n' && character != '\r' && character != '\t' &&
			(unicode.IsControl(character) || unicode.In(character, unicode.Cf)) {
			character = '\uFFFD'
		}
		length := utf8.RuneLen(character)
		if result.Len()+length > maximumBytes {
			break
		}
		result.WriteRune(character)
	}
	if result.Len() == 0 {
		return nil
	}
	text := result.String()
	return &text
}

func executionIdentity(lease submissionports.JobLease) supervisor.ExecutionIdentity {
	return supervisor.ExecutionIdentity{
		JobID:        lease.Job.ID,
		AttemptID:    lease.Attempt.ID,
		WorkerBootID: lease.Attempt.WorkerBootID,
	}
}

func digestText(value [32]byte) string {
	return fmt.Sprintf("sha256:%x", value[:])
}

func phaseTransitionIncident(phase string) submissionports.InfrastructureIncident {
	return submissionports.InfrastructureIncident{
		Classification: "infrastructure",
		Code:           "phase_" + phase + "_failed",
		Summary:        "the durable judge phase transition did not commit",
	}
}

func boundedMilliseconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Millisecond)
}

func boundedKilobytes(value int64) int {
	if value <= 0 {
		return 0
	}
	return int((value + 1023) >> 10)
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func clearJudgeCasePayload(payload *submissionports.JudgeCasePayload) {
	if payload == nil {
		return
	}
	clear(payload.Input.Content)
	clear(payload.Expected.Content)
	payload.Input.Content = nil
	payload.Expected.Content = nil
}

func clearCaseResult(result *executor.CaseResult) {
	if result == nil {
		return
	}
	clear(result.Output)
	clear(result.Execution.Stdout)
	clear(result.Execution.Stderr)
	clear(result.Execution.OutputArchive)
	result.Output = nil
	result.Execution.Stdout = nil
	result.Execution.Stderr = nil
	result.Execution.OutputArchive = nil
}

func clearCompileOutcome(outcome *executor.CompileOutcome) {
	if outcome == nil {
		return
	}
	clear(outcome.Diagnostic)
	clear(outcome.Execution.Stdout)
	clear(outcome.Execution.Stderr)
	clear(outcome.Execution.OutputArchive)
	outcome.Diagnostic = nil
	outcome.Execution.Stdout = nil
	outcome.Execution.Stderr = nil
	outcome.Execution.OutputArchive = nil
}

func validateCompileOutcome(
	outcome executor.CompileOutcome,
	lease submissionports.JobLease,
	profile problementity.RuntimeProfile,
) (submissionports.InfrastructureIncident, error) {
	evidenceIncident := submissionports.InfrastructureIncident{
		Classification: "integrity",
		Code:           "compile_evidence_invalid",
		Summary:        "sandbox compilation evidence failed trusted validation",
	}
	if !validTrustedExecution(outcome.Execution) {
		return evidenceIncident, ErrUntrustedExecution
	}

	limitExceeded := outcome.Execution.OOMKilled ||
		outcome.Execution.MemoryLimitExceeded ||
		outcome.Execution.ProcessLimitExceeded ||
		outcome.Execution.OutputLimitExceeded ||
		outcome.Execution.CPUTimeLimitExceeded ||
		outcome.Execution.WallTimeLimitExceeded
	switch outcome.Status {
	case executor.CompileStatusRejected:
		if outcome.Artifact != nil || limitExceeded ||
			(outcome.Execution.ExitCode == 0 && outcome.Execution.Signal == "") {
			return evidenceIncident, ErrUntrustedExecution
		}
	case executor.CompileStatusLimitExceeded:
		if outcome.Artifact != nil || !limitExceeded {
			return evidenceIncident, ErrUntrustedExecution
		}
	case executor.CompileStatusSucceeded:
		if outcome.Execution.ExitCode != 0 || outcome.Execution.Signal != "" ||
			limitExceeded || len(outcome.Execution.OutputArchive) != 0 {
			return evidenceIncident, ErrUntrustedExecution
		}
		if outcome.Artifact == nil {
			return submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "compiled_artifact_missing",
				Summary:        "successful compilation produced no staged artifact",
			}, executor.ErrCompileInfrastructure
		}
		if outcome.Artifact.AttemptID != lease.Attempt.ID ||
			outcome.Artifact.ManifestChecksum != profile.ManifestChecksum ||
			outcome.Artifact.RuntimeProfile.ManifestChecksum != profile.ManifestChecksum ||
			!reflect.DeepEqual(outcome.Artifact.RuntimeProfile, profile) {
			return submissionports.InfrastructureIncident{
				Classification: "integrity",
				Code:           "compiled_artifact_mismatch",
				Summary:        "compiled artifact escaped its exact lease and runtime contract",
			}, executor.ErrInvalidArtifact
		}
	default:
		return submissionports.InfrastructureIncident{
			Classification: "integrity",
			Code:           "compile_status_invalid",
			Summary:        "the compiler returned an unknown status",
		}, ErrLeaseExecutorInvalid
	}
	return submissionports.InfrastructureIncident{}, nil
}

func nilLeaseExecutionDependency(value any) bool {
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

var _ DurableLeaseExecutor = (*LeaseExecutor)(nil)
var _ CompiledArtifactReleaser = StagedArtifactReleaser{}
