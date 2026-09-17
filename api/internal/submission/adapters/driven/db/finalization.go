package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

type finalizationSQLDatabase interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type FinalizationRepository struct {
	db      finalizationSQLDatabase
	newID   func() uuid.UUID
	journal finalizationJournal
}

func NewFinalizationRepository(
	database finalizationSQLDatabase,
) (*FinalizationRepository, error) {
	if database == nil {
		return nil, ports.ErrFinalizationUnavailable
	}
	return &FinalizationRepository{
		db:      database,
		newID:   uuid.New,
		journal: newDurableFinalizationJournal(),
	}, nil
}

// NewFinalizationRepositoryFromEnt borrows the process SQL pool and never
// closes it. All finalization mutations use one caller-owned raw transaction.
func NewFinalizationRepositoryFromEnt(
	client *dbent.EntClient,
) (*FinalizationRepository, error) {
	if client == nil {
		return nil, ports.ErrFinalizationUnavailable
	}
	driver, ok := client.DialectDriver()
	if !ok {
		return nil, ports.ErrFinalizationUnavailable
	}
	sqlDriver, ok := driver.(*entsql.Driver)
	if !ok || sqlDriver.DB() == nil {
		return nil, ports.ErrFinalizationUnavailable
	}
	return NewFinalizationRepository(sqlDriver.DB())
}

type lockedFinalization struct {
	jobID             uuid.UUID
	submissionID      uuid.UUID
	generation        int64
	judgeSpecID       uuid.UUID
	jobState          string
	jobLeaseOwner     sql.NullString
	jobLeaseTokenHash []byte
	jobLeaseUntil     sql.NullTime
	originCID         string

	attemptID             uuid.UUID
	attemptJobID          uuid.UUID
	attemptSubmissionID   uuid.UUID
	attemptGeneration     int64
	attemptJudgeSpecID    uuid.UUID
	attemptState          string
	attemptLeaseOwner     string
	attemptLeaseTokenHash []byte
	workerID              uuid.UUID
	workerBootID          uuid.UUID
	workerBootCurrent     bool

	userID                        uuid.UUID
	problemID                     uuid.UUID
	problemRevisionID             uuid.UUID
	testsetRevisionID             uuid.UUID
	judgeSpecChecksum             []byte
	rewardSnapshot                json.RawMessage
	rewardSnapshotChecksum        []byte
	runtimeProfileVersion         string
	compilerVersion               string
	runtimeBootstrapVersion       string
	supervisorProtocolVersion     string
	sandboxImageDigest            string
	artifactManifestChecksum      string
	releaseID                     string
	approvedManifestChecksum      string
	checkerImplementationChecksum string

	submissionVersion    int64
	latestGeneration     int64
	latestPhase          string
	latestOutcome        sql.NullString
	activeJudgementID    uuid.NullUUID
	activeGeneration     sql.NullInt64
	activeVerdict        sql.NullString
	submittedAt          time.Time
	contestID            uuid.NullUUID
	contestProblemID     uuid.NullUUID
	contestPolicyVersion sql.NullInt64
	contestElapsedMS     sql.NullInt64
	contestSequence      sql.NullInt64
	verdictVersion       int64
}

func (repository *FinalizationRepository) FinalizeContestantEvaluation(
	ctx context.Context,
	request ports.FinalizationRequest,
) (ports.FinalizationResult, error) {
	if repository == nil || repository.db == nil || repository.newID == nil ||
		repository.journal == nil || ctx == nil ||
		request.ExpectedSubmissionVersion < 1 ||
		request.Judgement.Validate() != nil ||
		!request.Judgement.Verdict.Contestant() {
		return ports.FinalizationResult{}, ports.ErrInvalidFinalization
	}
	tx, err := repository.db.BeginTx(
		ctx,
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return ports.FinalizationResult{}, fmt.Errorf(
			"begin contestant finalization: %w",
			err,
		)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockRewardTimelineForTransition(ctx, tx); err != nil {
		return ports.FinalizationResult{}, err
	}
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	locked, err := lockFinalization(ctx, tx, request.Lease)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if locked.jobState == string(entity.JobDone) {
		result, duplicateErr := repository.readIdempotentFinalization(
			ctx,
			tx,
			locked,
			request.Lease,
			request.Judgement,
			request.TestResults,
			false,
		)
		if duplicateErr != nil {
			return ports.FinalizationResult{}, duplicateErr
		}
		if err := tx.Commit(); err != nil {
			return ports.FinalizationResult{}, fmt.Errorf(
				"commit contestant finalization replay: %w",
				err,
			)
		}
		return result, nil
	}
	if err := validateLiveFinalizationFence(
		locked,
		request.Lease,
		request.ExpectedSubmissionVersion,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	judgement, err := canonicalJudgement(
		locked,
		request.Lease,
		request.Judgement,
		now,
	)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := validatePersistedFinalizationResults(
		ctx,
		tx,
		locked,
		judgement,
		request.TestResults,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := repository.insertTestResults(
		ctx,
		tx,
		request.Lease,
		request.TestResults,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := markAttemptSucceeded(
		ctx,
		tx,
		request.Lease.Attempt.ID,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := insertJudgement(ctx, tx, judgement); err != nil {
		return ports.FinalizationResult{}, err
	}
	previous := locked.submissionState()
	next, err := entity.FinishGeneration(previous, entity.GenerationResult{
		Generation: judgement.Generation,
		Outcome:    string(judgement.Verdict),
		Judgement:  &judgement,
	})
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := updateContestantSubmission(
		ctx,
		tx,
		locked,
		judgement,
		next,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := adjustAcceptedCounters(
		ctx,
		tx,
		locked,
		judgement.Verdict,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := repository.rebuildContestStanding(
		ctx,
		tx,
		locked,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	transition, solveEvent, err := repository.reconcileSolve(
		ctx,
		tx,
		locked,
		now,
	)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := completeFinalizationJob(
		ctx,
		tx,
		request.Lease.Job.ID,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := releaseFinalizationCapacity(
		ctx,
		tx,
		request.Lease,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	events, entry, err := repository.contestantJournalRecords(
		ctx,
		locked,
		previous,
		next,
		judgement,
		now,
		solveEvent,
	)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := repository.journal.Append(ctx, tx, events, entry); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.FinalizationResult{}, fmt.Errorf(
			"commit contestant finalization: %w",
			err,
		)
	}
	result := ports.FinalizationResult{
		SubmissionID:         locked.submissionID,
		JudgementID:          judgement.ID,
		AggregateVersion:     next.AggregateVersion,
		ActiveVerdictChanged: true,
		FinalizedAt:          now,
		State:                next,
	}
	if transition != nil {
		transitionID := transition.ID
		result.SolveTransitionID = &transitionID
	}
	return result, nil
}

func lockRewardTimelineForTransition(ctx context.Context, tx *sql.Tx) error {
	var ignored any
	if err := tx.QueryRowContext(
		ctx,
		"SELECT pg_advisory_xact_lock_shared($1)",
		platformtx.RewardTimelineAdvisoryLock,
	).Scan(&ignored); err != nil {
		return fmt.Errorf("lock reward timeline for solve transition: %w", err)
	}
	return nil
}

func (repository *FinalizationRepository) FinalizeSystemError(
	ctx context.Context,
	request ports.FatalGenerationRequest,
) (ports.FinalizationResult, error) {
	if repository == nil || repository.db == nil || repository.newID == nil ||
		repository.journal == nil || ctx == nil ||
		request.ExpectedSubmissionVersion < 1 ||
		request.Judgement.Validate() != nil ||
		request.Judgement.Verdict != entity.VerdictSystemError {
		return ports.FinalizationResult{}, ports.ErrInvalidFinalization
	}
	tx, err := repository.db.BeginTx(
		ctx,
		&sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return ports.FinalizationResult{}, fmt.Errorf(
			"begin system-error finalization: %w",
			err,
		)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	locked, err := lockFinalization(ctx, tx, request.Lease)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if locked.jobState == string(entity.JobDead) {
		result, duplicateErr := repository.readIdempotentFinalization(
			ctx,
			tx,
			locked,
			request.Lease,
			request.Judgement,
			nil,
			true,
		)
		if duplicateErr != nil {
			return ports.FinalizationResult{}, duplicateErr
		}
		if err := tx.Commit(); err != nil {
			return ports.FinalizationResult{}, fmt.Errorf(
				"commit system-error finalization replay: %w",
				err,
			)
		}
		return result, nil
	}
	if err := validateLiveFinalizationFence(
		locked,
		request.Lease,
		request.ExpectedSubmissionVersion,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	judgement, err := canonicalJudgement(
		locked,
		request.Lease,
		request.Judgement,
		now,
	)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := markAttemptInfrastructureFailed(
		ctx,
		tx,
		request.Lease.Attempt.ID,
		request.Failure,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := insertJudgement(ctx, tx, judgement); err != nil {
		return ports.FinalizationResult{}, err
	}
	previous := locked.submissionState()
	next, err := entity.FinishGeneration(previous, entity.GenerationResult{
		Generation: judgement.Generation,
		Outcome:    string(entity.VerdictSystemError),
		Judgement:  &judgement,
	})
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := updateSystemErrorSubmission(
		ctx,
		tx,
		locked,
		judgement,
		next,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := deadFinalizationJob(
		ctx,
		tx,
		request.Lease.Job.ID,
		request.Failure,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := releaseFinalizationCapacity(
		ctx,
		tx,
		request.Lease,
		now,
	); err != nil {
		return ports.FinalizationResult{}, err
	}
	events, entry, err := repository.systemErrorJournalRecords(
		ctx,
		locked,
		previous,
		next,
		judgement,
		request.Failure,
		now,
	)
	if err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := repository.journal.Append(ctx, tx, events, entry); err != nil {
		return ports.FinalizationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ports.FinalizationResult{}, fmt.Errorf(
			"commit system-error finalization: %w",
			err,
		)
	}
	return ports.FinalizationResult{
		SubmissionID:     locked.submissionID,
		JudgementID:      judgement.ID,
		AggregateVersion: next.AggregateVersion,
		FinalizedAt:      now,
		State:            next,
	}, nil
}

func lockFinalization(
	ctx context.Context,
	tx *sql.Tx,
	lease ports.JobLease,
) (lockedFinalization, error) {
	if tx == nil || lease.Job.ID == uuid.Nil ||
		lease.Attempt.ID == uuid.Nil {
		return lockedFinalization{}, ports.ErrInvalidFinalization
	}
	var value lockedFinalization
	err := tx.QueryRowContext(ctx, `
		SELECT
			job.id, job.submission_id, job.generation, job.judge_spec_id,
			job.state, job.lease_owner, job.lease_token_hash, job.lease_until,
			job.origin_cid,
			attempt.id, attempt.job_id, attempt.submission_id,
			attempt.generation, attempt.judge_spec_id, attempt.state,
			attempt.lease_owner, attempt.lease_token_hash,
			attempt.worker_id, attempt.worker_boot_id,
			NOT EXISTS (
				SELECT 1
				FROM judge_worker_instances AS newer
				WHERE newer.worker_key = worker.worker_key
					AND (
						newer.boot_started_at > worker.boot_started_at
						OR (
							newer.boot_started_at = worker.boot_started_at
							AND newer.id > worker.id
						)
					)
			) AS worker_boot_current,
			spec.user_id, spec.problem_id, spec.problem_revision_id,
			spec.testset_revision_id, spec.spec_checksum,
			spec.reward_snapshot, spec.reward_snapshot_checksum,
			spec.runtime_profile_version, spec.compiler_version,
			spec.runtime_bootstrap_version,
			spec.supervisor_protocol_version, spec.sandbox_image_digest,
			spec.artifact_manifest_checksum, spec.release_id,
			spec.approved_release_manifest_checksum, spec.checker_implementation_checksum,
			submission.version, submission.latest_generation,
			submission.latest_generation_phase,
			submission.latest_generation_outcome,
			submission.active_judgement_id,
			submission.active_judgement_generation,
			submission.active_verdict,
			submission.submitted_at, submission.contest_id,
			submission.contest_problem_id,
			submission.contest_scoring_policy_version,
			submission.contest_elapsed_ms,
			submission.contest_submission_sequence,
			submission.verdict_version
		FROM judge_jobs AS job
		JOIN judge_attempts AS attempt
			ON attempt.job_id = job.id
			AND attempt.id = $2
		JOIN judge_specs AS spec
			ON spec.id = job.judge_spec_id
		JOIN submissions AS submission
			ON submission.id = job.submission_id
		JOIN judge_worker_instances AS worker
			ON worker.id = attempt.worker_id
			AND worker.boot_id = attempt.worker_boot_id
		WHERE job.id = $1
		FOR UPDATE OF job, attempt, submission`,
		lease.Job.ID,
		lease.Attempt.ID,
	).Scan(
		&value.jobID,
		&value.submissionID,
		&value.generation,
		&value.judgeSpecID,
		&value.jobState,
		&value.jobLeaseOwner,
		&value.jobLeaseTokenHash,
		&value.jobLeaseUntil,
		&value.originCID,
		&value.attemptID,
		&value.attemptJobID,
		&value.attemptSubmissionID,
		&value.attemptGeneration,
		&value.attemptJudgeSpecID,
		&value.attemptState,
		&value.attemptLeaseOwner,
		&value.attemptLeaseTokenHash,
		&value.workerID,
		&value.workerBootID,
		&value.workerBootCurrent,
		&value.userID,
		&value.problemID,
		&value.problemRevisionID,
		&value.testsetRevisionID,
		&value.judgeSpecChecksum,
		&value.rewardSnapshot,
		&value.rewardSnapshotChecksum,
		&value.runtimeProfileVersion,
		&value.compilerVersion,
		&value.runtimeBootstrapVersion,
		&value.supervisorProtocolVersion,
		&value.sandboxImageDigest,
		&value.artifactManifestChecksum,
		&value.releaseID,
		&value.approvedManifestChecksum,
		&value.checkerImplementationChecksum,
		&value.submissionVersion,
		&value.latestGeneration,
		&value.latestPhase,
		&value.latestOutcome,
		&value.activeJudgementID,
		&value.activeGeneration,
		&value.activeVerdict,
		&value.submittedAt,
		&value.contestID,
		&value.contestProblemID,
		&value.contestPolicyVersion,
		&value.contestElapsedMS,
		&value.contestSequence,
		&value.verdictVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return lockedFinalization{}, ports.ErrLeaseLost
	}
	if err != nil {
		return lockedFinalization{}, fmt.Errorf(
			"lock finalization lease: %w",
			err,
		)
	}
	if err := value.validateIdentity(lease); err != nil {
		return lockedFinalization{}, err
	}
	return value, nil
}

func (locked lockedFinalization) validateIdentity(
	lease ports.JobLease,
) error {
	if locked.jobID != lease.Job.ID ||
		locked.submissionID != lease.Job.SubmissionID ||
		locked.generation != lease.Job.Generation ||
		locked.judgeSpecID != lease.Job.JudgeSpecID ||
		locked.attemptID != lease.Attempt.ID ||
		locked.attemptJobID != lease.Job.ID ||
		locked.attemptSubmissionID != lease.Job.SubmissionID ||
		locked.attemptGeneration != lease.Job.Generation ||
		locked.attemptJudgeSpecID != lease.Job.JudgeSpecID ||
		locked.workerID != lease.Attempt.WorkerID ||
		locked.workerBootID != lease.Attempt.WorkerBootID ||
		locked.originCID != lease.Job.OriginCID ||
		locked.userID != lease.JudgeSpec.UserID ||
		locked.problemRevisionID != lease.JudgeSpec.ProblemRevisionID ||
		locked.testsetRevisionID != lease.JudgeSpec.TestsetRevisionID ||
		locked.judgeSpecID != lease.JudgeSpec.ID ||
		locked.runtimeProfileVersion != lease.JudgeSpec.RuntimeProfileVersion ||
		locked.compilerVersion != lease.JudgeSpec.CompilerVersion ||
		locked.runtimeBootstrapVersion != lease.JudgeSpec.RuntimeBootstrapVersion ||
		locked.supervisorProtocolVersion != lease.JudgeSpec.SupervisorProtocol ||
		locked.sandboxImageDigest != lease.JudgeSpec.SandboxImageDigest ||
		locked.artifactManifestChecksum != lease.JudgeSpec.ArtifactManifestChecksum ||
		locked.releaseID != lease.JudgeSpec.ReleaseID ||
		locked.approvedManifestChecksum != lease.JudgeSpec.ApprovedManifestChecksum ||
		locked.checkerImplementationChecksum != lease.JudgeSpec.CheckerImplementationChecksum ||
		!bytes.Equal(
			locked.judgeSpecChecksum,
			lease.JudgeSpec.SpecChecksum,
		) ||
		!bytes.Equal(
			locked.rewardSnapshotChecksum,
			lease.JudgeSpec.RewardSnapshotChecksum,
		) ||
		len(locked.judgeSpecChecksum) != sha256.Size ||
		len(locked.rewardSnapshotChecksum) != sha256.Size ||
		locked.problemID == uuid.Nil ||
		locked.userID == uuid.Nil {
		return ports.ErrLeaseLost
	}
	return nil
}

func validateLiveFinalizationFence(
	locked lockedFinalization,
	lease ports.JobLease,
	expectedVersion int64,
	now time.Time,
) error {
	if locked.jobState != string(entity.JobLeased) ||
		locked.attemptState != string(entity.AttemptFinalizing) ||
		!locked.jobLeaseOwner.Valid ||
		!locked.jobLeaseUntil.Valid ||
		!constantTimeTextEqual(
			locked.jobLeaseOwner.String,
			lease.LeaseOwner,
		) ||
		!constantTimeTextEqual(
			locked.attemptLeaseOwner,
			lease.LeaseOwner,
		) ||
		len(lease.LeaseToken) != leaseTokenBytes ||
		len(locked.jobLeaseTokenHash) != sha256.Size ||
		len(locked.attemptLeaseTokenHash) != sha256.Size ||
		!locked.workerBootCurrent ||
		!locked.jobLeaseUntil.Time.After(now) ||
		locked.latestGeneration != locked.generation ||
		locked.latestPhase != string(entity.PhaseFinalizing) ||
		locked.latestOutcome.Valid ||
		locked.submissionVersion != expectedVersion {
		return ports.ErrLeaseLost
	}
	hash := sha256.Sum256(lease.LeaseToken)
	if subtle.ConstantTimeCompare(
		hash[:],
		locked.jobLeaseTokenHash,
	) != 1 || subtle.ConstantTimeCompare(
		hash[:],
		locked.attemptLeaseTokenHash,
	) != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func canonicalJudgement(
	locked lockedFinalization,
	lease ports.JobLease,
	request entity.Judgement,
	now time.Time,
) (entity.Judgement, error) {
	if request.ID == uuid.Nil ||
		request.SubmissionID != locked.submissionID ||
		request.Generation != locked.generation ||
		request.JudgeSpecID != locked.judgeSpecID ||
		request.ProblemRevisionID != locked.problemRevisionID ||
		request.TestsetRevisionID != locked.testsetRevisionID {
		return entity.Judgement{}, ports.ErrInvalidFinalization
	}
	if request.WinningAttemptID != nil &&
		*request.WinningAttemptID != lease.Attempt.ID {
		return entity.Judgement{}, ports.ErrInvalidFinalization
	}
	judgement := request
	copy(judgement.JudgeSpecChecksum[:], locked.judgeSpecChecksum)
	judgement.RewardSnapshot = append(
		json.RawMessage(nil),
		locked.rewardSnapshot...,
	)
	copy(
		judgement.RewardSnapshotChecksum[:],
		locked.rewardSnapshotChecksum,
	)
	judgement.RuntimeProfileVersion = locked.runtimeProfileVersion
	judgement.CompilerVersion = locked.compilerVersion
	judgement.RuntimeBootstrapVersion = locked.runtimeBootstrapVersion
	judgement.SupervisorProtocolVersion = locked.supervisorProtocolVersion
	judgement.SandboxImageDigest = locked.sandboxImageDigest
	judgement.ArtifactManifestChecksum = locked.artifactManifestChecksum
	judgement.ReleaseID = locked.releaseID
	judgement.ApprovedReleaseManifestChecksum = locked.approvedManifestChecksum
	judgement.CheckerImplementationChecksum = locked.checkerImplementationChecksum
	judgement.CompletedAt = now.UTC()
	judgement.OriginCID = locked.originCID
	if judgement.Verdict.Contestant() {
		attemptID := lease.Attempt.ID
		judgement.WinningAttemptID = &attemptID
	}
	if judgement.Validate() != nil {
		return entity.Judgement{}, ports.ErrInvalidFinalization
	}
	return judgement, nil
}

func (locked lockedFinalization) submissionState() entity.SubmissionState {
	state := entity.SubmissionState{
		ID:               locked.submissionID,
		LatestGeneration: locked.latestGeneration,
		LatestPhase:      entity.Phase(locked.latestPhase),
		AggregateVersion: locked.submissionVersion,
	}
	if locked.latestOutcome.Valid {
		state.LatestOutcome = locked.latestOutcome.String
	}
	if locked.activeJudgementID.Valid {
		value := locked.activeJudgementID.UUID
		state.ActiveJudgementID = &value
	}
	if locked.activeGeneration.Valid {
		value := locked.activeGeneration.Int64
		state.ActiveGeneration = &value
	}
	if locked.activeVerdict.Valid {
		value := entity.Verdict(locked.activeVerdict.String)
		state.ActiveVerdict = &value
	}
	return state
}

func (repository *FinalizationRepository) insertTestResults(
	ctx context.Context,
	tx *sql.Tx,
	lease ports.JobLease,
	results []ports.JudgeTestResult,
	now time.Time,
) error {
	if len(results) == 0 {
		return nil
	}
	if len(results) > 256 {
		return ports.ErrInvalidFinalization
	}
	ordered := append([]ports.JudgeTestResult(nil), results...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].ExecutionOrder <
			ordered[right].ExecutionOrder
	})
	const columnsPerResult = 23
	arguments := make([]any, 0, len(ordered)*columnsPerResult)
	identities := make(map[uuid.UUID]struct{}, len(ordered))
	var statement strings.Builder
	statement.WriteString(`
		INSERT INTO judge_test_results (
			id, created_at, attempt_id, judge_spec_id,
			testset_revision_id, group_id, group_index,
			group_stop_on_failure, testcase_revision_id, case_order,
			execution_order, test_kind, verdict, time_ms, memory_kb,
			exit_code, terminating_signal, stdout_bytes, stderr_bytes,
			safe_diagnostic, checker_metadata, execution_started_at,
			execution_completed_at
		) VALUES `)
	for index, result := range ordered {
		resultID := repository.newID()
		if resultID == uuid.Nil {
			return ports.ErrFinalizationUnavailable
		}
		if _, duplicate := identities[resultID]; duplicate {
			return ports.ErrFinalizationUnavailable
		}
		identities[resultID] = struct{}{}
		if index != 0 {
			statement.WriteByte(',')
		}
		statement.WriteByte('(')
		for column := 0; column < columnsPerResult; column++ {
			if column != 0 {
				statement.WriteByte(',')
			}
			statement.WriteByte('$')
			statement.WriteString(
				strconv.Itoa(index*columnsPerResult + column + 1),
			)
		}
		statement.WriteByte(')')
		arguments = append(
			arguments,
			resultID,
			now,
			lease.Attempt.ID,
			result.JudgeSpecID,
			result.TestsetRevisionID,
			result.GroupID,
			result.GroupIndex,
			result.StopOnFailure,
			result.TestcaseRevisionID,
			result.CaseOrder,
			result.ExecutionOrder,
			result.Kind,
			string(result.Verdict),
			result.TimeMilliseconds,
			result.MemoryKilobytes,
			result.ExitCode,
			result.TerminatingSignal,
			result.StdoutBytes,
			result.StderrBytes,
			result.SafeDiagnostic,
			result.CheckerMetadata,
			result.ExecutionStartedAt.UTC(),
			result.ExecutionEndedAt.UTC(),
		)
	}
	if _, err := tx.ExecContext(
		ctx,
		statement.String(),
		arguments...,
	); err != nil {
		return fmt.Errorf("insert immutable judge test results: %w", err)
	}
	return nil
}

func markAttemptSucceeded(
	ctx context.Context,
	tx *sql.Tx,
	attemptID uuid.UUID,
	now time.Time,
) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET state = 'succeeded', heartbeat_at = $2,
			finished_at = $2, updated_at = $2
		WHERE id = $1 AND state = 'finalizing'`,
		attemptID,
		now,
	)
	if err != nil {
		return fmt.Errorf("complete judge attempt: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func markAttemptInfrastructureFailed(
	ctx context.Context,
	tx *sql.Tx,
	attemptID uuid.UUID,
	failure ports.InfrastructureFailure,
	now time.Time,
) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET state = 'infrastructure_failed',
			failure_classification = $2,
			failure_code = $3, failure_summary = $4,
			heartbeat_at = $5, finished_at = $5, updated_at = $5
		WHERE id = $1 AND state = 'finalizing'`,
		attemptID,
		failure.Classification,
		failure.Code,
		failure.Summary,
		now,
	)
	if err != nil {
		return fmt.Errorf("fail judge attempt: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func insertJudgement(
	ctx context.Context,
	tx *sql.Tx,
	judgement entity.Judgement,
) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO submission_judgements (
			id, created_at, submission_id, generation, judge_spec_id,
			problem_revision_id, testset_revision_id, verdict,
			passed_count, total_count, maximum_time_ms,
			maximum_memory_kb, compile_diagnostic_summary,
			judge_spec_checksum, reward_snapshot,
			reward_snapshot_checksum, runtime_profile_version,
			compiler_version, runtime_bootstrap_version,
			supervisor_protocol_version, sandbox_image_digest,
			artifact_manifest_checksum, winning_attempt_id,
			release_id, approved_release_manifest_checksum,
			checker_implementation_checksum, completed_at, origin_cid
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25, $26, $27, $28
		)`,
		judgement.ID,
		judgement.CompletedAt,
		judgement.SubmissionID,
		judgement.Generation,
		judgement.JudgeSpecID,
		judgement.ProblemRevisionID,
		judgement.TestsetRevisionID,
		string(judgement.Verdict),
		judgement.PassedCount,
		judgement.TotalCount,
		judgement.MaximumTimeMilliseconds,
		judgement.MaximumMemoryKilobytes,
		judgement.CompileDiagnosticSummary,
		judgement.JudgeSpecChecksum[:],
		judgement.RewardSnapshot,
		judgement.RewardSnapshotChecksum[:],
		judgement.RuntimeProfileVersion,
		judgement.CompilerVersion,
		judgement.RuntimeBootstrapVersion,
		judgement.SupervisorProtocolVersion,
		judgement.SandboxImageDigest,
		judgement.ArtifactManifestChecksum,
		judgement.WinningAttemptID,
		judgement.ReleaseID,
		judgement.ApprovedReleaseManifestChecksum,
		judgement.CheckerImplementationChecksum,
		judgement.CompletedAt,
		judgement.OriginCID,
	)
	if err != nil {
		return fmt.Errorf("insert immutable submission judgement: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrFinalizationConflict
	}
	return nil
}
