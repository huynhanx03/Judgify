package db

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/correlation"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const (
	rejudgeAuditAction          = "submission.rejudge"
	cancelAuditAction           = "submission.cancel"
	rejudgeJobNamespace         = "judgify/submission-rejudge-job/v1"
	maximumOperationReasonBytes = 1024
)

var ErrInvalidSubmissionOperations = errors.New("submission operations: invalid repository operation")

// SubmissionOperationsRepository owns administrative mutation transactions.
// It deliberately uses raw SQL for the complete lock/clone/update sequence:
// Ent models the rows, but the security property is the one SQL transaction
// across submission state, immutable spec, job, event and audit log.
type SubmissionOperationsRepository struct {
	client       *dbent.EntClient
	transactions platformtx.UnitOfWork
	outbox       eventing.OutboxAppender
	audit        audit.Appender
	maxAttempts  uint32
}

func NewSubmissionOperationsRepository(
	client *dbent.EntClient,
	transactions platformtx.UnitOfWork,
	outbox eventing.OutboxAppender,
	auditAppender audit.Appender,
	maxAttempts uint32,
) (*SubmissionOperationsRepository, error) {
	if client == nil || transactions == nil || outbox == nil || auditAppender == nil ||
		maxAttempts < 1 || maxAttempts > maximumInitialJudgeAttempts {
		return nil, ErrInvalidSubmissionOperations
	}
	return &SubmissionOperationsRepository{
		client: client, transactions: transactions, outbox: outbox,
		audit: auditAppender, maxAttempts: maxAttempts,
	}, nil
}

func (repository *SubmissionOperationsRepository) Rejudge(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.RejudgeCommand,
) (ports.OperationResult, error) {
	if repository == nil || repository.client == nil || repository.transactions == nil ||
		repository.outbox == nil || repository.audit == nil || ctx == nil ||
		!validOperationActorRecord(actor) || !validRejudgeRecord(command) {
		return ports.OperationResult{}, ErrInvalidSubmissionOperations
	}
	ctx = correlation.EnsureContext(ctx)
	var result ports.OperationResult
	err := repository.transactions.Within(ctx, func(txCtx context.Context) error {
		client, err := platformtx.Client(txCtx, nil)
		if err != nil {
			return err
		}
		now, err := operationDatabaseNow(txCtx, client)
		if err != nil {
			return err
		}
		jobID := deterministicRejudgeID(command)
		if err := takeOperationCommandLock(txCtx, client, jobID); err != nil {
			return err
		}
		if replay, found, replayErr := readRejudgeReplay(txCtx, client, jobID, command.SubmissionID); replayErr != nil {
			return replayErr
		} else if found {
			result = replay
			return nil
		}

		locked, err := lockOperationSubmission(txCtx, client, command.SubmissionID)
		if err != nil {
			return err
		}
		if locked.version != command.ExpectedVersion ||
			(locked.phase != string(entity.PhaseTerminal) && locked.phase != string(entity.PhaseCancelled)) {
			return ports.ErrOperationConflict
		}
		spec, err := lockGenerationSpec(txCtx, client, command.SubmissionID, locked.generation)
		if err != nil {
			return err
		}
		if !spec.matchesChecksum(command.SubmissionID, locked.generation) {
			return ErrInvalidSubmissionOperations
		}
		nextGeneration := locked.generation + 1
		nextVersion := locked.version + 1
		if nextGeneration < 2 || nextVersion < 2 {
			return ErrInvalidSubmissionOperations
		}
		nextSpecID := derivedOperationID(jobID, "spec")
		specChecksum, err := spec.checksum(command.SubmissionID, nextGeneration)
		if err != nil {
			return fmt.Errorf("compute rejudge specification checksum: %w", err)
		}
		if err := insertRejudgeSpec(txCtx, client, nextSpecID, command.SubmissionID, nextGeneration, specChecksum, spec, now); err != nil {
			return err
		}
		if err := insertRejudgeJob(txCtx, client, jobID, nextSpecID, command.SubmissionID, nextGeneration, "maintenance", repository.maxAttempts, correlation.FromContext(txCtx), now); err != nil {
			return err
		}
		updated, err := client.Submission.ExecContext(txCtx, `
			UPDATE submissions
			SET latest_generation = $2, latest_generation_phase = 'queued',
				latest_generation_outcome = NULL, latest_generation_queued_at = $3,
				latest_generation_started_at = NULL, latest_generation_completed_at = NULL,
				version = $4, updated_at = $3
			WHERE id = $1 AND version = $5 AND latest_generation = $6
				AND latest_generation_phase IN ('terminal','cancelled')`,
			command.SubmissionID, nextGeneration, now, nextVersion,
			command.ExpectedVersion, locked.generation,
		)
		if err != nil {
			return fmt.Errorf("queue rejudge generation: %w", err)
		}
		if affected, affectErr := updated.RowsAffected(); affectErr != nil || affected != 1 {
			return ports.ErrOperationConflict
		}
		if err := repository.appendRejudgeJournal(txCtx, actor, command, jobID, nextSpecID, "maintenance", nextGeneration, nextVersion, now); err != nil {
			return err
		}
		result = ports.OperationResult{
			SubmissionID: command.SubmissionID, Generation: nextGeneration,
			AggregateVersion: nextVersion, Phase: entity.PhaseQueued, JobID: &jobID,
		}
		return nil
	})
	if err != nil {
		return ports.OperationResult{}, err
	}
	return result, nil
}

func (repository *SubmissionOperationsRepository) Cancel(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.CancelCommand,
) (ports.OperationResult, error) {
	if repository == nil || repository.client == nil || repository.transactions == nil ||
		repository.outbox == nil || repository.audit == nil || ctx == nil ||
		!validOperationActorRecord(actor) || !validCancelRecord(command) {
		return ports.OperationResult{}, ErrInvalidSubmissionOperations
	}
	ctx = correlation.EnsureContext(ctx)
	var result ports.OperationResult
	err := repository.transactions.Within(ctx, func(txCtx context.Context) error {
		client, err := platformtx.Client(txCtx, nil)
		if err != nil {
			return err
		}
		now, err := operationDatabaseNow(txCtx, client)
		if err != nil {
			return err
		}
		locked, err := lockOperationSubmission(txCtx, client, command.SubmissionID)
		if err != nil {
			return err
		}
		if locked.generation != command.Generation {
			return ports.ErrOperationConflict
		}
		if locked.phase == string(entity.PhaseCancelled) {
			if locked.version < command.ExpectedVersion {
				return ports.ErrOperationConflict
			}
			result = ports.OperationResult{SubmissionID: command.SubmissionID, Generation: command.Generation, AggregateVersion: locked.version, Phase: entity.PhaseCancelled, Outcome: "cancelled", IdempotentReplay: true}
			return nil
		}
		if locked.version != command.ExpectedVersion || locked.phase == string(entity.PhaseTerminal) {
			return ports.ErrOperationConflict
		}
		job, err := lockGenerationJob(txCtx, client, command.SubmissionID, command.Generation)
		if err != nil {
			return err
		}
		if job.state != string(entity.JobReady) && job.state != string(entity.JobRetryWait) && job.state != string(entity.JobLeased) {
			return ports.ErrOperationConflict
		}
		// A leased job remains fenced until its owner observes the cancelled
		// generation through Heartbeat. Clearing its lease here would turn a
		// deliberate cancellation into an indistinguishable lease-loss race.
		if job.state == string(entity.JobReady) || job.state == string(entity.JobRetryWait) {
			updated, updateErr := client.Submission.ExecContext(txCtx, `
				UPDATE judge_jobs
				SET state = 'cancelled', cancelled_at = $2, updated_at = $2
				WHERE id = $1 AND state IN ('ready','retry_wait')`, job.id, now)
			if updateErr != nil {
				return fmt.Errorf("cancel queued judge job: %w", updateErr)
			}
			if affected, affectErr := updated.RowsAffected(); affectErr != nil || affected != 1 {
				return ports.ErrOperationConflict
			}
		}
		nextVersion := locked.version + 1
		updated, err := client.Submission.ExecContext(txCtx, `
			UPDATE submissions
			SET latest_generation_phase = 'cancelled', latest_generation_outcome = 'cancelled',
				latest_generation_completed_at = $2, version = $3, updated_at = $2
			WHERE id = $1 AND version = $4 AND latest_generation = $5
				AND latest_generation_phase IN ('queued','claimed','compiling','running','finalizing')
				AND latest_generation_outcome IS NULL`,
			command.SubmissionID, now, nextVersion, command.ExpectedVersion, command.Generation,
		)
		if err != nil {
			return fmt.Errorf("cancel submission generation: %w", err)
		}
		if affected, affectErr := updated.RowsAffected(); affectErr != nil || affected != 1 {
			return ports.ErrOperationConflict
		}
		if err := repository.appendCancelJournal(txCtx, actor, command, job.id, nextVersion, now); err != nil {
			return err
		}
		result = ports.OperationResult{SubmissionID: command.SubmissionID, Generation: command.Generation, AggregateVersion: nextVersion, Phase: entity.PhaseCancelled, Outcome: "cancelled", JobID: &job.id}
		return nil
	})
	if err != nil {
		return ports.OperationResult{}, err
	}
	return result, nil
}

type lockedOperationSubmission struct {
	version    int64
	generation int64
	phase      string
}

type lockedOperationJob struct {
	id    uuid.UUID
	state string
}

type rejudgeSpec struct {
	userID                          uuid.UUID
	problemID                       uuid.UUID
	language                        string
	runtimeKey                      string
	runtimeProfileVersion           string
	compilerVersion                 string
	runtimeBootstrapVersion         string
	sandboxImageDigest              string
	compileCommandID                string
	runCommandID                    string
	artifactManifestVersion         string
	artifactManifestChecksum        string
	artifactManifest                json.RawMessage
	supervisorProtocolVersion       string
	checkerKey                      string
	checkerVersion                  string
	checkerConfig                   json.RawMessage
	releaseID                       string
	approvedReleaseManifestChecksum string
	checkerImplementationChecksum   string
	capabilityProfileHash           string
	rewardSnapshot                  json.RawMessage
	rewardSnapshotChecksum          []byte
	cpuTimeLimitMS                  int
	wallTimeLimitMS                 int
	memoryLimitKB                   int
	outputLimitBytes                int
	processLimit                    int
	compileCPUTimeLimitMS           int
	compileWallTimeLimitMS          int
	totalAttemptWallTimeLimitMS     int
	sourceSHA256                    []byte
	judgeSpecProtocol               string
	specChecksum                    []byte
	problemRevisionID               uuid.UUID
	testsetRevisionID               uuid.UUID
}

func (spec rejudgeSpec) checksum(submissionID uuid.UUID, generation int64) ([32]byte, error) {
	rewardChecksum, err := operationDigest(spec.rewardSnapshotChecksum)
	if err != nil {
		return [32]byte{}, err
	}
	sourceChecksum, err := operationDigest(spec.sourceSHA256)
	if err != nil {
		return [32]byte{}, err
	}
	return submissioncontract.ComputeJudgeSpecChecksum(submissioncontract.JudgeSpecChecksumInput{
		SubmissionID: submissionID, UserID: spec.userID, ProblemID: spec.problemID, Generation: generation,
		ProblemRevisionID: spec.problemRevisionID, TestsetRevisionID: spec.testsetRevisionID,
		Language: spec.language, RuntimeKey: spec.runtimeKey, RuntimeProfileVersion: spec.runtimeProfileVersion,
		CompilerVersion: spec.compilerVersion, RuntimeBootstrapVersion: spec.runtimeBootstrapVersion,
		SandboxImageDigest: spec.sandboxImageDigest, CompileCommandID: spec.compileCommandID, RunCommandID: spec.runCommandID,
		ArtifactManifestVersion: spec.artifactManifestVersion, ArtifactManifestChecksum: spec.artifactManifestChecksum,
		ArtifactManifest: append(json.RawMessage(nil), spec.artifactManifest...), SupervisorProtocol: spec.supervisorProtocolVersion,
		CheckerKey: spec.checkerKey, CheckerVersion: spec.checkerVersion, CheckerConfig: append(json.RawMessage(nil), spec.checkerConfig...),
		ReleaseID:                       spec.releaseID,
		ApprovedReleaseManifestChecksum: spec.approvedReleaseManifestChecksum,
		CheckerImplementationChecksum:   spec.checkerImplementationChecksum,
		CapabilityProfileHash:           spec.capabilityProfileHash, RewardSnapshot: append(json.RawMessage(nil), spec.rewardSnapshot...),
		RewardSnapshotChecksum: rewardChecksum, CPUTimeMilliseconds: spec.cpuTimeLimitMS, WallTimeMilliseconds: spec.wallTimeLimitMS,
		MemoryKilobytes: spec.memoryLimitKB, OutputBytes: spec.outputLimitBytes, ProcessCount: spec.processLimit,
		CompileCPUMilliseconds: spec.compileCPUTimeLimitMS, CompileWallMilliseconds: spec.compileWallTimeLimitMS,
		TotalWallMilliseconds: spec.totalAttemptWallTimeLimitMS, SourceSHA256: sourceChecksum, JudgeSpecProtocol: spec.judgeSpecProtocol,
	})
}

func (spec rejudgeSpec) matchesChecksum(
	submissionID uuid.UUID,
	generation int64,
) bool {
	stored, err := operationDigest(spec.specChecksum)
	if err != nil {
		return false
	}
	computed, err := spec.checksum(submissionID, generation)
	return err == nil && subtle.ConstantTimeCompare(stored[:], computed[:]) == 1
}

func operationDigest(value []byte) ([32]byte, error) {
	if len(value) != sha256.Size {
		return [32]byte{}, ErrInvalidSubmissionOperations
	}
	var result [32]byte
	copy(result[:], value)
	if result == ([32]byte{}) {
		return [32]byte{}, ErrInvalidSubmissionOperations
	}
	return result, nil
}

func operationDatabaseNow(ctx context.Context, client *generate.Client) (time.Time, error) {
	rows, err := client.Submission.QueryContext(ctx, `SELECT clock_timestamp()`)
	if err != nil {
		return time.Time{}, fmt.Errorf("read operation database clock: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return time.Time{}, err
		}
		return time.Time{}, ErrInvalidSubmissionOperations
	}
	var now time.Time
	if err := rows.Scan(&now); err != nil || now.IsZero() {
		return time.Time{}, ErrInvalidSubmissionOperations
	}
	if rows.Next() || rows.Err() != nil {
		return time.Time{}, ErrInvalidSubmissionOperations
	}
	return now.UTC(), nil
}

// takeOperationCommandLock serializes exact-command replays before the first
// receipt lookup. Without it, two concurrent requests could both observe no
// deterministic job and one would incorrectly receive a stale-state conflict
// instead of the committed idempotent result.
func takeOperationCommandLock(ctx context.Context, client *generate.Client, jobID uuid.UUID) error {
	if jobID == uuid.Nil {
		return ErrInvalidSubmissionOperations
	}
	rows, err := client.Submission.QueryContext(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1, 7415110))`, jobID.String())
	if err != nil {
		return fmt.Errorf("lock rejudge command: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrInvalidSubmissionOperations
	}
	if rows.Err() != nil || rows.Next() || rows.Err() != nil {
		return ErrInvalidSubmissionOperations
	}
	return nil
}

func readRejudgeReplay(
	ctx context.Context,
	client *generate.Client,
	jobID uuid.UUID,
	submissionID uuid.UUID,
) (ports.OperationResult, bool, error) {
	rows, err := client.Submission.QueryContext(ctx, `
		SELECT job.submission_id, job.generation, submission.version,
			submission.latest_generation_phase, COALESCE(submission.latest_generation_outcome, '')
		FROM judge_jobs AS job
		JOIN submissions AS submission ON submission.id = job.submission_id
		WHERE job.id = $1
		FOR SHARE OF job, submission`, jobID)
	if err != nil {
		return ports.OperationResult{}, false, fmt.Errorf("read rejudge replay: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return ports.OperationResult{}, false, rows.Err()
	}
	var storedSubmissionID uuid.UUID
	var result ports.OperationResult
	var phase string
	if err := rows.Scan(&storedSubmissionID, &result.Generation, &result.AggregateVersion, &phase, &result.Outcome); err != nil {
		return ports.OperationResult{}, false, fmt.Errorf("scan rejudge replay: %w", err)
	}
	if rows.Next() || rows.Err() != nil {
		return ports.OperationResult{}, false, ErrInvalidSubmissionOperations
	}
	if storedSubmissionID != submissionID {
		return ports.OperationResult{}, false, ports.ErrOperationConflict
	}
	result.SubmissionID = submissionID
	result.Phase = entity.Phase(phase)
	result.JobID = &jobID
	result.IdempotentReplay = true
	return result, true, nil
}

func lockOperationSubmission(
	ctx context.Context,
	client *generate.Client,
	submissionID uuid.UUID,
) (lockedOperationSubmission, error) {
	rows, err := client.Submission.QueryContext(ctx, `
		SELECT submission.version, submission.latest_generation,
			submission.latest_generation_phase
		FROM submissions AS submission
		WHERE submission.id = $1
		FOR UPDATE OF submission`, submissionID)
	if err != nil {
		return lockedOperationSubmission{}, fmt.Errorf("lock submission operation: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return lockedOperationSubmission{}, err
		}
		return lockedOperationSubmission{}, ports.ErrOperationConflict
	}
	var value lockedOperationSubmission
	if err := rows.Scan(&value.version, &value.generation, &value.phase); err != nil {
		return lockedOperationSubmission{}, fmt.Errorf("scan locked submission operation: %w", err)
	}
	if rows.Next() || rows.Err() != nil || value.version < 1 || value.generation < 1 {
		return lockedOperationSubmission{}, ErrInvalidSubmissionOperations
	}
	return value, nil
}

func lockGenerationJob(
	ctx context.Context,
	client *generate.Client,
	submissionID uuid.UUID,
	generation int64,
) (lockedOperationJob, error) {
	rows, err := client.Submission.QueryContext(ctx, `
		SELECT id, state FROM judge_jobs
		WHERE submission_id = $1 AND generation = $2
		FOR UPDATE`, submissionID, generation)
	if err != nil {
		return lockedOperationJob{}, fmt.Errorf("lock submission judge job: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return lockedOperationJob{}, err
		}
		return lockedOperationJob{}, ports.ErrOperationConflict
	}
	var value lockedOperationJob
	if err := rows.Scan(&value.id, &value.state); err != nil || value.id == uuid.Nil {
		return lockedOperationJob{}, ErrInvalidSubmissionOperations
	}
	if rows.Next() || rows.Err() != nil {
		return lockedOperationJob{}, ErrInvalidSubmissionOperations
	}
	return value, nil
}

func lockGenerationSpec(
	ctx context.Context,
	client *generate.Client,
	submissionID uuid.UUID,
	generation int64,
) (rejudgeSpec, error) {
	rows, err := client.Submission.QueryContext(ctx, `
		SELECT spec.user_id, spec.problem_id, spec.problem_revision_id,
			spec.testset_revision_id, spec.language, spec.runtime_key,
			spec.runtime_profile_version, spec.compiler_version,
			spec.runtime_bootstrap_version, spec.sandbox_image_digest,
			spec.compile_command_id, spec.run_command_id,
			spec.artifact_manifest_version, spec.artifact_manifest_checksum,
			spec.artifact_manifest, spec.supervisor_protocol_version,
			spec.checker_key, spec.checker_version, spec.checker_config,
			spec.release_id, spec.approved_release_manifest_checksum,
			spec.checker_implementation_checksum, spec.capability_profile_hash,
			spec.reward_snapshot, spec.reward_snapshot_checksum,
			spec.cpu_time_limit_ms, spec.wall_time_limit_ms,
			spec.memory_limit_kb, spec.output_limit_bytes, spec.process_limit,
			spec.compile_cpu_time_limit_ms, spec.compile_wall_time_limit_ms,
			spec.total_attempt_wall_time_limit_ms, spec.source_sha256,
			spec.judge_spec_protocol, spec.spec_checksum
		FROM judge_specs AS spec
		JOIN activated_judge_capabilities AS activation
		  ON activation.release_id = spec.release_id
		 AND activation.approved_release_manifest_checksum =
			spec.approved_release_manifest_checksum
		 AND activation.state = 'active'
		 AND activation.retired_at IS NULL
		 AND activation.judge_spec_protocol = spec.judge_spec_protocol
		 AND activation.runtime_key = spec.runtime_key
		 AND activation.runtime_profile_version = spec.runtime_profile_version
		 AND activation.compiler_version = spec.compiler_version
		 AND activation.runtime_bootstrap_version =
			spec.runtime_bootstrap_version
		 AND activation.compile_command_id = spec.compile_command_id
		 AND activation.run_command_id = spec.run_command_id
		 AND activation.image_digest = spec.sandbox_image_digest
		 AND activation.manifest_version = spec.artifact_manifest_version
		 AND activation.manifest_checksum = spec.artifact_manifest_checksum
		 AND activation.supervisor_protocol =
			spec.supervisor_protocol_version
		 AND activation.checker_key = spec.checker_key
		 AND activation.checker_version = spec.checker_version
		 AND activation.checker_implementation_checksum =
			spec.checker_implementation_checksum
		 AND activation.capability_profile_hash = spec.capability_profile_hash
		WHERE spec.submission_id = $1 AND spec.generation = $2
		-- judge_specs is append-only; locking it is neither permitted nor needed.
		-- activated_judge_capabilities is mutable (state transitions) and must be
		-- locked to confirm the activation is still active.
		FOR SHARE OF activation`, submissionID, generation)
	if err != nil {
		return rejudgeSpec{}, fmt.Errorf("lock previous judge specification: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return rejudgeSpec{}, err
		}
		return rejudgeSpec{}, ports.ErrOperationConflict
	}
	var value rejudgeSpec
	if err := rows.Scan(
		&value.userID, &value.problemID, &value.problemRevisionID, &value.testsetRevisionID, &value.language,
		&value.runtimeKey, &value.runtimeProfileVersion, &value.compilerVersion,
		&value.runtimeBootstrapVersion, &value.sandboxImageDigest, &value.compileCommandID,
		&value.runCommandID, &value.artifactManifestVersion, &value.artifactManifestChecksum,
		&value.artifactManifest, &value.supervisorProtocolVersion, &value.checkerKey, &value.checkerVersion,
		&value.checkerConfig, &value.releaseID,
		&value.approvedReleaseManifestChecksum,
		&value.checkerImplementationChecksum,
		&value.capabilityProfileHash, &value.rewardSnapshot,
		&value.rewardSnapshotChecksum, &value.cpuTimeLimitMS, &value.wallTimeLimitMS,
		&value.memoryLimitKB, &value.outputLimitBytes, &value.processLimit,
		&value.compileCPUTimeLimitMS, &value.compileWallTimeLimitMS,
		&value.totalAttemptWallTimeLimitMS, &value.sourceSHA256, &value.judgeSpecProtocol,
		&value.specChecksum,
	); err != nil {
		return rejudgeSpec{}, fmt.Errorf("scan previous judge specification: %w", err)
	}
	if rows.Next() || rows.Err() != nil {
		return rejudgeSpec{}, ErrInvalidSubmissionOperations
	}
	return value, nil
}

func insertRejudgeSpec(
	ctx context.Context,
	client *generate.Client,
	specID uuid.UUID,
	submissionID uuid.UUID,
	generation int64,
	checksum [32]byte,
	spec rejudgeSpec,
	now time.Time,
) error {
	if specID == uuid.Nil || submissionID == uuid.Nil || generation < 2 || checksum == ([32]byte{}) {
		return ErrInvalidSubmissionOperations
	}
	result, err := client.Submission.ExecContext(ctx, `
		INSERT INTO judge_specs (
			id, created_at, submission_id, user_id, problem_id, generation,
			problem_revision_id, testset_revision_id, language, runtime_key,
			runtime_profile_version, compiler_version, runtime_bootstrap_version,
			sandbox_image_digest, compile_command_id, run_command_id,
			artifact_manifest_version, artifact_manifest_checksum, artifact_manifest,
			supervisor_protocol_version, checker_key, checker_version, checker_config,
			release_id, approved_release_manifest_checksum,
			checker_implementation_checksum, capability_profile_hash,
			reward_snapshot, reward_snapshot_checksum,
			cpu_time_limit_ms, wall_time_limit_ms, memory_limit_kb, output_limit_bytes,
			process_limit, compile_cpu_time_limit_ms, compile_wall_time_limit_ms,
			total_attempt_wall_time_limit_ms, source_sha256, judge_spec_protocol,
			spec_checksum
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19::jsonb, $20, $21, $22, $23::jsonb,
			$24, $25, $26, $27, $28::jsonb, $29, $30, $31, $32, $33,
			$34, $35, $36, $37, $38, $39, $40
		)`,
		specID, now, submissionID, spec.userID, spec.problemID, generation,
		spec.problemRevisionID, spec.testsetRevisionID, spec.language, spec.runtimeKey,
		spec.runtimeProfileVersion, spec.compilerVersion, spec.runtimeBootstrapVersion,
		spec.sandboxImageDigest, spec.compileCommandID, spec.runCommandID,
		spec.artifactManifestVersion, spec.artifactManifestChecksum, spec.artifactManifest,
		spec.supervisorProtocolVersion, spec.checkerKey, spec.checkerVersion, spec.checkerConfig,
		spec.releaseID, spec.approvedReleaseManifestChecksum,
		spec.checkerImplementationChecksum, spec.capabilityProfileHash,
		spec.rewardSnapshot, spec.rewardSnapshotChecksum,
		spec.cpuTimeLimitMS, spec.wallTimeLimitMS, spec.memoryLimitKB, spec.outputLimitBytes,
		spec.processLimit, spec.compileCPUTimeLimitMS, spec.compileWallTimeLimitMS,
		spec.totalAttemptWallTimeLimitMS, spec.sourceSHA256, spec.judgeSpecProtocol, checksum[:],
	)
	if err != nil {
		return fmt.Errorf("insert rejudge specification: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ErrInvalidSubmissionOperations
	}
	return nil
}

func insertRejudgeJob(
	ctx context.Context,
	client *generate.Client,
	jobID uuid.UUID,
	specID uuid.UUID,
	submissionID uuid.UUID,
	generation int64,
	lane string,
	maxAttempts uint32,
	originCID string,
	now time.Time,
) error {
	if jobID == uuid.Nil || specID == uuid.Nil || submissionID == uuid.Nil ||
		generation < 2 || maxAttempts < 1 || originCID == "" ||
		(lane != "contest" && lane != "regular" && lane != "maintenance") {
		return ErrInvalidSubmissionOperations
	}
	result, err := client.Submission.ExecContext(ctx, `
		INSERT INTO judge_jobs (
			id, created_at, updated_at, submission_id, generation, judge_spec_id,
			lane, base_priority, state, available_at, attempts, max_attempts, origin_cid
		) VALUES ($1, $2, $2, $3, $4, $5, $6, 0, 'ready', $2, 0, $7, $8)`,
		jobID, now, submissionID, generation, specID, lane, maxAttempts, originCID,
	)
	if err != nil {
		return fmt.Errorf("insert rejudge job: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ErrInvalidSubmissionOperations
	}
	return nil
}

func (repository *SubmissionOperationsRepository) appendRejudgeJournal(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.RejudgeCommand,
	jobID uuid.UUID,
	specID uuid.UUID,
	lane string,
	generation int64,
	version int64,
	now time.Time,
) error {
	eventID := derivedOperationID(jobID, "event")
	payload, err := json.Marshal(submissioncontract.SubmissionQueuedV1{
		SubmissionID: command.SubmissionID, Generation: generation, JudgeSpecID: specID,
		JobID: jobID, Lane: lane, QueuedAt: now.UTC().Format(time.RFC3339Nano),
		CID: correlation.FromContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("encode rejudge queued event: %w", err)
	}
	if err := repository.outbox.Append(ctx, eventing.Event{
		ID: eventID, Type: submissioncontract.EventSubmissionQueuedV1,
		AggregateType: submissioncontract.AggregateSubmission, AggregateID: command.SubmissionID,
		AggregateVersion: version, CorrelationID: correlation.FromContext(ctx), Payload: payload, OccurredAt: now,
	}); err != nil {
		return fmt.Errorf("append rejudge outbox event: %w", err)
	}
	if _, err := repository.audit.Append(ctx, audit.Entry{
		ID: derivedOperationID(jobID, "audit"), ActorUserID: &actor.UserID, ActorSessionID: actor.SessionID,
		Action: rejudgeAuditAction, Resource: submissioncontract.AggregateSubmission,
		ResourceID: command.SubmissionID.String(), Reason: command.Reason,
		Before:        map[string]any{"generation": generation - 1, "version": version - 1},
		After:         map[string]any{"generation": generation, "version": version, "job_id": jobID.String(), "judge_spec_id": specID.String()},
		CorrelationID: correlation.FromContext(ctx), OccurredAt: now,
	}); err != nil {
		return fmt.Errorf("append rejudge audit: %w", err)
	}
	return nil
}

func (repository *SubmissionOperationsRepository) appendCancelJournal(
	ctx context.Context,
	actor ports.OperationActor,
	command ports.CancelCommand,
	jobID uuid.UUID,
	version int64,
	now time.Time,
) error {
	eventID := derivedOperationID(jobID, "cancel-event")
	payload, err := json.Marshal(struct {
		SubmissionID uuid.UUID `json:"submission_id"`
		Generation   int64     `json:"generation"`
		JobID        uuid.UUID `json:"job_id"`
		Phase        string    `json:"phase"`
		OccurredAt   string    `json:"occurred_at"`
		CID          string    `json:"cid"`
	}{
		SubmissionID: command.SubmissionID, Generation: command.Generation, JobID: jobID,
		Phase: string(entity.PhaseCancelled), OccurredAt: now.UTC().Format(time.RFC3339Nano), CID: correlation.FromContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("encode submission cancellation event: %w", err)
	}
	if err := repository.outbox.Append(ctx, eventing.Event{
		ID: eventID, Type: submissioncontract.EventSubmissionCancelledV1,
		AggregateType: submissioncontract.AggregateSubmission, AggregateID: command.SubmissionID,
		AggregateVersion: version, CorrelationID: correlation.FromContext(ctx), Payload: payload, OccurredAt: now,
	}); err != nil {
		return fmt.Errorf("append cancellation outbox event: %w", err)
	}
	if _, err := repository.audit.Append(ctx, audit.Entry{
		ID: derivedOperationID(jobID, "cancel-audit"), ActorUserID: &actor.UserID, ActorSessionID: actor.SessionID,
		Action: cancelAuditAction, Resource: submissioncontract.AggregateSubmission,
		ResourceID: command.SubmissionID.String(), Reason: command.Reason,
		Before:        map[string]any{"generation": command.Generation, "version": version - 1, "phase": "active"},
		After:         map[string]any{"generation": command.Generation, "version": version, "phase": string(entity.PhaseCancelled), "job_id": jobID.String()},
		CorrelationID: correlation.FromContext(ctx), OccurredAt: now,
	}); err != nil {
		return fmt.Errorf("append cancellation audit: %w", err)
	}
	return nil
}

func deterministicRejudgeID(command ports.RejudgeCommand) uuid.UUID {
	encoded := make([]byte, 0, len(rejudgeJobNamespace)+16+16+8+len(command.Reason))
	encoded = append(encoded, rejudgeJobNamespace...)
	encoded = append(encoded, command.SubmissionID[:]...)
	encoded = append(encoded, command.IdempotencyKey[:]...)
	var version [8]byte
	binary.BigEndian.PutUint64(version[:], uint64(command.ExpectedVersion))
	encoded = append(encoded, version[:]...)
	encoded = append(encoded, command.Reason...)
	return uuid.NewSHA1(uuid.NameSpaceOID, encoded)
}

func derivedOperationID(parent uuid.UUID, purpose string) uuid.UUID {
	return uuid.NewSHA1(parent, []byte(purpose))
}

func validOperationActorRecord(actor ports.OperationActor) bool {
	return actor.UserID != uuid.Nil && (actor.SessionID == nil || *actor.SessionID != uuid.Nil)
}

func validRejudgeRecord(command ports.RejudgeCommand) bool {
	return command.SubmissionID != uuid.Nil && command.ExpectedVersion > 0 && command.IdempotencyKey != uuid.Nil && validOperationReasonRecord(command.Reason)
}

func validCancelRecord(command ports.CancelCommand) bool {
	return command.SubmissionID != uuid.Nil && command.Generation > 0 && command.ExpectedVersion > 0 && validOperationReasonRecord(command.Reason)
}

func validOperationReasonRecord(reason string) bool {
	if len(reason) == 0 || len(reason) > maximumOperationReasonBytes || strings.TrimSpace(reason) == "" || !utf8.ValidString(reason) {
		return false
	}
	for _, character := range reason {
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

var _ ports.SubmissionOperationsRepository = (*SubmissionOperationsRepository)(nil)
