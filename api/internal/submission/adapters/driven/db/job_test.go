package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestClaimCommitsExactCapabilityLeaseAttemptAndSubmissionTogether(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	rawToken := bytes.Repeat([]byte{0x42}, leaseTokenBytes)
	tokenHash := sha256.Sum256(rawToken)
	repository.random = bytes.NewReader(rawToken)
	attemptID := testutil.ID(90)
	repository.newID = func() uuid.UUID { return attemptID }
	candidate := jobClaimTestCandidate(now)
	request := jobClaimTestRequest()

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	expectCurrentClaimWorker(mock, now, 4)
	mock.ExpectQuery(exactClaimQueryPattern()).
		WithArgs(
			now,
			"contest",
			request.WorkerID,
			request.WorkerBootID,
			request.GlobalRunningLimit,
			request.PerUserRunningLimit,
		).
		WillReturnRows(candidate.rows())
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'leased'`).
		WithArgs(
			candidate.jobID,
			request.LeaseOwner,
			exactByteArgument(tokenHash[:]),
			now.Add(request.LeaseDuration),
			now,
			uint32(0),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO judge_attempts`).
		WithArgs(
			attemptID,
			now,
			candidate.jobID,
			candidate.submissionID,
			int64(1),
			candidate.specID,
			uint32(1),
			request.LeaseOwner,
			exactByteArgument(tokenHash[:]),
			request.WorkerID,
			request.WorkerBootID,
			candidate.originCID,
			candidate.runtimeProfileVersion,
			candidate.compilerVersion,
			candidate.runtimeBootstrapVersion,
			candidate.supervisorProtocol,
			candidate.imageDigest,
			candidate.manifestChecksum,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE submissions\s+SET latest_generation_phase = 'claimed'`).
		WithArgs(candidate.submissionID, int64(1), int64(7), now).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(int64(8)))
	mock.ExpectExec(`UPDATE judge_worker_instances\s+SET free_capacity = free_capacity -`).
		WithArgs(request.WorkerID, request.WorkerBootID, 1, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	leases, err := repository.Claim(context.Background(), request)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if len(leases) != 1 {
		t.Fatalf("Claim() leases = %d, want 1", len(leases))
	}
	lease := leases[0]
	if lease.Job.ID != candidate.jobID || lease.Attempt.ID != attemptID ||
		lease.LeaseOwner != request.LeaseOwner ||
		!bytes.Equal(lease.LeaseToken, rawToken) ||
		lease.Attempt.LeaseTokenHash != entity.LeaseTokenHash(tokenHash) ||
		lease.JudgeSpec.CompileCommandID != candidate.compileCommandID ||
		lease.JudgeSpec.RunCommandID != candidate.runCommandID ||
		lease.SubmissionVersion != 8 ||
		!lease.LeaseUntil.Equal(now.Add(request.LeaseDuration)) {
		t.Fatalf("Claim() lease = %#v, want exact committed claim", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestClaimRollsBackJobMutationWhenAttemptInsertFails(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 5, 0, 0, time.UTC)
	tokenReader := &capturingLeaseReader{value: 0x24}
	repository.random = tokenReader
	repository.newID = func() uuid.UUID { return testutil.ID(91) }
	candidate := jobClaimTestCandidate(now)
	request := jobClaimTestRequest()
	insertErr := errors.New("attempt insert failed")

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	expectCurrentClaimWorker(mock, now, 1)
	mock.ExpectQuery(exactClaimQueryPattern()).
		WillReturnRows(candidate.rows())
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'leased'`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO judge_attempts`).WillReturnError(insertErr)
	mock.ExpectRollback()

	leases, err := repository.Claim(context.Background(), request)
	if !errors.Is(err, insertErr) {
		t.Fatalf("Claim() error = %v, want insert failure", err)
	}
	if leases != nil {
		t.Fatalf("Claim() returned leases after rollback: %#v", leases)
	}
	for index, value := range tokenReader.target {
		if value != 0 {
			t.Fatalf("rolled-back raw token byte %d was not wiped", index)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestClaimDeniesNonCurrentWorkerBootBeforeSelectingAnyJob(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 10, 0, 0, time.UTC)
	request := jobClaimTestRequest()

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	mock.ExpectQuery(`SELECT worker\.claims_enabled.*NOT EXISTS.*newer\.worker_key`).
		WillReturnRows(sqlmock.NewRows([]string{
			"claims_enabled", "heartbeat_at", "free_capacity",
		}))
	mock.ExpectRollback()

	leases, err := repository.Claim(context.Background(), request)
	if !errors.Is(err, ports.ErrClaimDenied) {
		t.Fatalf("Claim() error = %v, want claim denied", err)
	}
	if leases != nil {
		t.Fatalf("Claim() leases = %#v, want nil", leases)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestHeartbeatRejectsWrongRawTokenBeforeAnyLeaseMutation(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 15, 0, 0, time.UTC)
	correctToken := bytes.Repeat([]byte{0x71}, leaseTokenBytes)
	wrongToken := bytes.Repeat([]byte{0x72}, leaseTokenBytes)
	correctHash := sha256.Sum256(correctToken)
	heartbeat := jobLeaseHeartbeat(wrongToken, now.Add(30*time.Second))

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectHeartbeatCurrentWorker(mock, now)
	mock.ExpectQuery(`SELECT lease_owner, lease_token_hash, lease_until`).
		WithArgs(heartbeat.JobID).
		WillReturnRows(sqlmock.NewRows([]string{
			"lease_owner", "lease_token_hash", "lease_until",
			"submission_id", "generation", "state",
		}).AddRow(
			"worker-a",
			correctHash[:],
			now.Add(time.Minute),
			testutil.ID(12),
			int64(1),
			"leased",
		))
	mock.ExpectRollback()

	_, err := repository.Heartbeat(context.Background(), heartbeat)
	if !errors.Is(err, ports.ErrLeaseLost) {
		t.Fatalf("Heartbeat() error = %v, want lease lost", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestHeartbeatExtendsOnlyExactCurrentAttemptAndGeneration(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 20, 0, 0, time.UTC)
	token := bytes.Repeat([]byte{0x31}, leaseTokenBytes)
	hash := sha256.Sum256(token)
	heartbeat := jobLeaseHeartbeat(token, now.Add(30*time.Second))
	submissionID := testutil.ID(12)

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectHeartbeatCurrentWorker(mock, now)
	mock.ExpectQuery(`SELECT lease_owner, lease_token_hash, lease_until`).
		WillReturnRows(sqlmock.NewRows([]string{
			"lease_owner", "lease_token_hash", "lease_until",
			"submission_id", "generation", "state",
		}).AddRow(
			"worker-a",
			hash[:],
			now.Add(time.Minute),
			submissionID,
			int64(1),
			"leased",
		))
	mock.ExpectQuery(`SELECT lease_owner, lease_token_hash, worker_id, worker_boot_id, state\s+FROM judge_attempts`).
		WithArgs(heartbeat.AttemptID, heartbeat.JobID, submissionID, int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"lease_owner", "lease_token_hash", "worker_id", "worker_boot_id", "state",
		}).AddRow(
			heartbeat.LeaseOwner,
			hash[:],
			heartbeat.WorkerID,
			heartbeat.WorkerBootID,
			"running",
		))
	mock.ExpectQuery(`SELECT latest_generation, latest_generation_phase`).
		WithArgs(submissionID).
		WillReturnRows(sqlmock.NewRows([]string{
			"latest_generation", "latest_generation_phase",
		}).AddRow(int64(1), "running"))
	mock.ExpectExec(`UPDATE judge_attempts\s+SET heartbeat_at`).
		WithArgs(
			heartbeat.AttemptID,
			heartbeat.JobID,
			heartbeat.WorkerID,
			heartbeat.WorkerBootID,
			heartbeat.LeaseOwner,
			exactByteArgument(hash[:]),
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE judge_jobs SET lease_until`).
		WithArgs(heartbeat.JobID, heartbeat.ExtendTo, now).
		WillReturnRows(
			sqlmock.NewRows([]string{"lease_until"}).
				AddRow(heartbeat.ExtendTo),
		)
	mock.ExpectCommit()

	acceptedUntil, err := repository.Heartbeat(context.Background(), heartbeat)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if !acceptedUntil.Equal(heartbeat.ExtendTo) {
		t.Fatalf(
			"Heartbeat() accepted deadline = %s, want %s",
			acceptedUntil,
			heartbeat.ExtendTo,
		)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestWorkerBootHeartbeatCannotReviveAnOlderBoot(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 25, 0, 0, time.UTC)
	heartbeat := ports.WorkerBootHeartbeat{
		WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
	}

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	mock.ExpectExec(`UPDATE judge_worker_instances AS worker.*NOT EXISTS`).
		WithArgs(heartbeat.WorkerID, heartbeat.WorkerBootID, now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repository.HeartbeatWorkerBoot(context.Background(), heartbeat)
	if !errors.Is(err, ports.ErrWorkerBootLost) {
		t.Fatalf("HeartbeatWorkerBoot() error = %v, want worker boot lost", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestWorkerBootHeartbeatReconcilesPersistedLeaseCapacityBeforeClaim(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 26, 0, 0, time.UTC)
	heartbeat := ports.WorkerBootHeartbeat{
		WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
	}
	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	mock.ExpectExec(`UPDATE judge_worker_instances AS worker.*free_capacity = GREATEST.*FROM judge_jobs AS job.*job\.state = 'leased'.*NOT EXISTS`).
		WithArgs(heartbeat.WorkerID, heartbeat.WorkerBootID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repository.HeartbeatWorkerBoot(context.Background(), heartbeat); err != nil {
		t.Fatalf("HeartbeatWorkerBoot() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestTransitionPhaseUsesTheSameLeaseFenceAndOptimisticSubmissionVersion(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 27, 0, 0, time.UTC)
	lease := jobRepositoryTestLease(now)

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectFencedLeaseWorker(mock, now)
	expectFencedLeaseRow(mock, lease, now, "claimed", "created", 8, 1, 5)
	mock.ExpectExec(`UPDATE judge_attempts\s+SET state = \$2`).
		WithArgs(lease.Attempt.ID, "compiling", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE submissions\s+SET latest_generation_phase = \$4`).
		WithArgs(
			lease.Job.SubmissionID,
			lease.Job.Generation,
			int64(8),
			"compiling",
			now,
			"claimed",
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "latest_generation", "latest_generation_phase",
			"latest_generation_outcome", "active_judgement_generation",
			"active_judgement_id", "active_verdict", "version",
		}).AddRow(
			lease.Job.SubmissionID,
			int64(1),
			"compiling",
			nil,
			nil,
			nil,
			nil,
			int64(9),
		))
	mock.ExpectCommit()

	state, err := repository.TransitionPhase(
		context.Background(),
		lease,
		8,
		entity.PhaseCompiling,
	)
	if err != nil {
		t.Fatalf("TransitionPhase() error = %v", err)
	}
	if state.ID != lease.Job.SubmissionID ||
		state.LatestPhase != entity.PhaseCompiling ||
		state.AggregateVersion != 9 {
		t.Fatalf("TransitionPhase() state = %#v, want compiling version 9", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRecordRetryClosesAttemptReturnsGenerationAndReleasesCapacityAtomically(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 28, 0, 0, time.UTC)
	lease := jobRepositoryTestLease(now)
	failure := ports.InfrastructureFailure{
		Classification: "infrastructure",
		Code:           "sandbox_start_failed",
		Summary:        "sandbox supervisor did not start",
		RetryAfter:     time.Second,
	}

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobBacklogAdmissionAdvisoryLock(mock)
	expectFencedLeaseWorker(mock, now)
	expectFencedLeaseRow(mock, lease, now, "running", "running", 9, 1, 5)
	mock.ExpectExec(`UPDATE judge_attempts\s+SET state = 'infrastructure_failed'`).
		WithArgs(
			lease.Attempt.ID,
			failure.Classification,
			failure.Code,
			failure.Summary,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'retry_wait'`).
		WithArgs(
			lease.Job.ID,
			now.Add(failure.RetryAfter),
			failure.Code,
			failure.Summary,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE submissions\s+SET latest_generation_phase = 'queued'`).
		WithArgs(lease.Job.SubmissionID, lease.Job.Generation, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE judge_worker_instances\s+SET free_capacity = LEAST`).
		WithArgs(lease.Attempt.WorkerID, lease.Attempt.WorkerBootID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repository.RecordRetry(context.Background(), lease, failure); err != nil {
		t.Fatalf("RecordRetry() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestRecordRetryRejectsUnboundedOrLogForgingCodesBeforeDatabaseAccess(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 29, 0, 0, time.UTC)
	lease := jobRepositoryTestLease(now)
	for _, failure := range []ports.InfrastructureFailure{
		{
			Classification: "infrastructure\nforged",
			Code:           "sandbox_start_failed",
			Summary:        "bounded summary", RetryAfter: time.Second,
		},
		{
			Classification: "infrastructure",
			Code:           "Sandbox Start Failed",
			Summary:        "bounded summary", RetryAfter: time.Second,
		},
	} {
		if err := repository.RecordRetry(context.Background(), lease, failure); !errors.Is(err, ErrInvalidJobRepository) {
			t.Fatalf("RecordRetry() error = %v, want invalid operation", err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected SQL: %v", err)
	}
}

func TestAcknowledgeCancellationClosesTheFencedLeaseWithoutRetryOrFinalization(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 29, 30, 0, time.UTC)
	lease := jobRepositoryTestLease(now)
	hash := sha256.Sum256(lease.LeaseToken)

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	mock.ExpectQuery(`SELECT job\.submission_id.*FOR UPDATE OF job, attempt, submission`).
		WithArgs(lease.Job.ID, lease.Attempt.ID).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "generation", "judge_spec_id", "job_state", "lease_owner",
			"job_lease_token_hash", "attempt_lease_token_hash", "worker_id", "worker_boot_id",
			"attempt_state", "phase",
		}).AddRow(
			lease.Job.SubmissionID,
			lease.Job.Generation,
			lease.Job.JudgeSpecID,
			"leased",
			lease.LeaseOwner,
			hash[:],
			hash[:],
			lease.Attempt.WorkerID,
			lease.Attempt.WorkerBootID,
			"running",
			"cancelled",
		))
	mock.ExpectExec(`UPDATE judge_attempts\s+SET state = 'cancelled'`).
		WithArgs(lease.Attempt.ID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'cancelled'`).
		WithArgs(lease.Job.ID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE judge_worker_instances\s+SET free_capacity = LEAST`).
		WithArgs(lease.Attempt.WorkerID, lease.Attempt.WorkerBootID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repository.AcknowledgeCancellation(context.Background(), lease); err != nil {
		t.Fatalf("AcknowledgeCancellation() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestAcknowledgeCancellationReplayDoesNotReleaseCapacityTwice(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 29, 31, 0, time.UTC)
	lease := jobRepositoryTestLease(now)
	hash := sha256.Sum256(lease.LeaseToken)

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	mock.ExpectQuery(`SELECT job\.submission_id.*FOR UPDATE OF job, attempt, submission`).
		WithArgs(lease.Job.ID, lease.Attempt.ID).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "generation", "judge_spec_id", "job_state", "lease_owner",
			"job_lease_token_hash", "attempt_lease_token_hash", "worker_id", "worker_boot_id",
			"attempt_state", "phase",
		}).AddRow(
			lease.Job.SubmissionID,
			lease.Job.Generation,
			lease.Job.JudgeSpecID,
			"cancelled",
			nil,
			nil,
			hash[:],
			lease.Attempt.WorkerID,
			lease.Attempt.WorkerBootID,
			"cancelled",
			"cancelled",
		))
	mock.ExpectCommit()

	if err := repository.AcknowledgeCancellation(context.Background(), lease); err != nil {
		t.Fatalf("AcknowledgeCancellation() replay error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

func TestReaperPromotesRetriesAndReconcilesExpiredRetryDeadAndCancelledLeases(t *testing.T) {
	repository, mock := newJobSQLRepository(t)
	now := time.Date(2026, time.July, 28, 12, 30, 0, 0, time.UTC)
	repository.random = bytes.NewReader(make([]byte, 256))
	repository.newID = func() uuid.UUID { return testutil.ID(99) }
	retry := expiredJobTestRow(1, "running", 1, 5)
	dead := expiredJobTestRow(2, "compiling", 5, 5)
	cancelled := expiredJobTestRow(3, "cancelled", 2, 5)

	mock.ExpectBegin()
	expectJobDatabaseNow(mock, now)
	expectJobClaimAdvisoryLock(mock)
	expectJobBacklogAdmissionAdvisoryLock(mock)
	mock.ExpectExec(`WITH promotable AS .*FOR UPDATE OF job SKIP LOCKED`).
		WithArgs(now, 32).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`SELECT j\.id, j\.submission_id, j\.generation.*FOR UPDATE OF j, a SKIP LOCKED`).
		WithArgs(now, 32).
		WillReturnRows(
			sqlmock.NewRows([]string{
				"job_id", "submission_id", "generation", "phase",
				"attempts", "max_attempts", "attempt_id", "worker_id", "worker_boot_id",
			}).
				AddRow(retry.values()...).
				AddRow(dead.values()...).
				AddRow(cancelled.values()...),
		)
	expectExpiredAttemptClosure(mock, retry, now)
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'retry_wait'`).
		WithArgs(retry.jobID, now, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE submissions\s+SET latest_generation_phase = 'queued'`).
		WithArgs(retry.submissionID, retry.generation, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectWorkerCapacityRelease(mock, retry, now)

	expectExpiredAttemptClosure(mock, dead, now)
	mock.ExpectQuery(`SELECT job\.origin_cid, spec\.user_id, spec\.problem_id.*FOR SHARE OF submission`).
		WithArgs(dead.jobID, dead.submissionID, dead.generation).
		WillReturnRows(sqlmock.NewRows([]string{
			"origin_cid", "user_id", "problem_id", "problem_revision_id",
			"testset_revision_id", "submitted_at", "version", "phase", "outcome",
			"active_judgement_id", "active_judgement_generation", "active_verdict",
		}).AddRow(
			testutil.ID(17).String(), testutil.ID(18), testutil.ID(19),
			testutil.ID(20), testutil.ID(21), now.Add(-time.Minute),
			int64(14), "compiling", nil, nil, nil, nil,
		))
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'dead'`).
		WithArgs(dead.jobID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO submission_judgements`).
		WithArgs(testutil.ID(99), now, dead.jobID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE submissions\s+SET latest_generation_phase = 'terminal'`).
		WithArgs(dead.submissionID, dead.generation, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO outbox_events`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectWorkerCapacityRelease(mock, dead, now)

	mock.ExpectExec(`UPDATE judge_attempts\s+SET state = 'cancelled'`).
		WithArgs(cancelled.attemptID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE judge_jobs\s+SET state = 'cancelled'`).
		WithArgs(cancelled.jobID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectWorkerCapacityRelease(mock, cancelled, now)
	mock.ExpectCommit()

	result, err := repository.ReapExpired(context.Background(), ports.ReapRequest{
		Limit: 32, RetryInitial: time.Second, RetryMaximum: time.Minute,
	})
	if err != nil {
		t.Fatalf("ReapExpired() error = %v", err)
	}
	want := ports.ReapResult{Promoted: 2, Retried: 1, Dead: 1, Cancelled: 1}
	if result != want {
		t.Fatalf("ReapExpired() = %#v, want %#v", result, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}

type exactByteArgument []byte

type capturingLeaseReader struct {
	value  byte
	target []byte
}

func (reader *capturingLeaseReader) Read(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = reader.value
	}
	reader.target = destination
	return len(destination), nil
}

func (expected exactByteArgument) Match(value driver.Value) bool {
	actual, ok := value.([]byte)
	return ok && bytes.Equal(actual, expected)
}

type claimTestCandidate struct {
	jobID                   uuid.UUID
	submissionID            uuid.UUID
	specID                  uuid.UUID
	userID                  uuid.UUID
	problemRevisionID       uuid.UUID
	testsetRevisionID       uuid.UUID
	originCID               string
	createdAt               time.Time
	runtimeProfileVersion   string
	compilerVersion         string
	runtimeBootstrapVersion string
	compileCommandID        string
	runCommandID            string
	imageDigest             string
	manifestChecksum        string
	supervisorProtocol      string
	releaseID               string
	approvedManifest        string
	checkerImplementation   string
}

func jobClaimTestCandidate(now time.Time) claimTestCandidate {
	return claimTestCandidate{
		jobID: testutil.ID(11), submissionID: testutil.ID(12),
		specID: testutil.ID(13), userID: testutil.ID(14),
		problemRevisionID: testutil.ID(15), testsetRevisionID: testutil.ID(16),
		originCID: testutil.ID(17).String(), createdAt: now.Add(-time.Minute),
		runtimeProfileVersion:   "go-1.26.5-v1",
		compilerVersion:         "go1.26.5",
		runtimeBootstrapVersion: "bootstrap-v1",
		compileCommandID:        "go.compile.v1",
		runCommandID:            "go.run.v1",
		imageDigest:             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		manifestChecksum:        "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		supervisorProtocol:      "supervisor-v1",
		releaseID:               "judge-release.test",
		approvedManifest:        "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		checkerImplementation:   "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	}
}

func (candidate claimTestCandidate) rows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"job_id", "submission_id", "generation", "judge_spec_id", "lane",
		"base_priority", "created_at", "attempts", "max_attempts", "origin_cid",
		"submission_version", "spec_id", "spec_submission_id", "spec_generation",
		"user_id", "problem_revision_id", "testset_revision_id", "judge_spec_protocol",
		"runtime_key", "runtime_profile_version", "compiler_version",
		"runtime_bootstrap_version", "compile_command_id", "run_command_id",
		"sandbox_image_digest",
		"artifact_manifest_version", "artifact_manifest_checksum",
		"supervisor_protocol_version", "checker_key", "checker_version",
		"release_id", "approved_release_manifest_checksum",
		"checker_implementation_checksum",
		"capability_profile_hash", "spec_checksum", "reward_snapshot_checksum",
	}).AddRow(
		candidate.jobID,
		candidate.submissionID,
		int64(1),
		candidate.specID,
		"contest",
		100,
		candidate.createdAt,
		uint32(0),
		uint32(5),
		candidate.originCID,
		int64(7),
		candidate.specID,
		candidate.submissionID,
		int64(1),
		candidate.userID,
		candidate.problemRevisionID,
		candidate.testsetRevisionID,
		"judge-spec-v1",
		"go",
		candidate.runtimeProfileVersion,
		candidate.compilerVersion,
		candidate.runtimeBootstrapVersion,
		candidate.compileCommandID,
		candidate.runCommandID,
		candidate.imageDigest,
		"manifest-v1",
		candidate.manifestChecksum,
		candidate.supervisorProtocol,
		"exact",
		"v1",
		candidate.releaseID,
		candidate.approvedManifest,
		candidate.checkerImplementation,
		"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		bytes.Repeat([]byte{0x11}, sha256.Size),
		bytes.Repeat([]byte{0x12}, sha256.Size),
	)
}

func jobClaimTestRequest() ports.JobClaimRequest {
	return ports.JobClaimRequest{
		WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
		LeaseOwner: "worker-a", LeaseDuration: 30 * time.Second,
		HeartbeatFreshness: 10 * time.Second, Capacity: 1,
		GlobalRunningLimit: 32, PerUserRunningLimit: 2,
	}
}

func jobLeaseHeartbeat(token []byte, extendTo time.Time) ports.LeaseHeartbeat {
	return ports.LeaseHeartbeat{
		JobID: testutil.ID(11), AttemptID: testutil.ID(90),
		WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
		LeaseOwner: "worker-a", LeaseToken: token, ExtendTo: extendTo,
	}
}

func jobRepositoryTestLease(now time.Time) ports.JobLease {
	token := bytes.Repeat([]byte{0x41}, leaseTokenBytes)
	hash := sha256.Sum256(token)
	jobID := testutil.ID(11)
	submissionID := testutil.ID(12)
	specID := testutil.ID(13)
	return ports.JobLease{
		Job: entity.JudgeJob{
			ID: jobID, SubmissionID: submissionID, Generation: 1,
			JudgeSpecID: specID, Lane: "regular", State: entity.JobLeased,
			Attempts: 1, MaxAttempts: 5, OriginCID: testutil.ID(17).String(),
		},
		Attempt: entity.JudgeAttempt{
			ID: testutil.ID(90), JobID: jobID, SubmissionID: submissionID,
			Generation: 1, JudgeSpecID: specID, AttemptNumber: 1,
			LeaseTokenHash: entity.LeaseTokenHash(hash),
			WorkerID:       testutil.ID(1), WorkerBootID: testutil.ID(2),
			State: entity.AttemptCreated, StartedAt: now,
		},
		JudgeSpec: ports.JudgeSpecClaim{
			ID: specID, SubmissionID: submissionID, Generation: 1,
			UserID: testutil.ID(14),
		},
		LeaseOwner: "worker-a", LeaseToken: token,
		LeaseUntil: now.Add(time.Minute), SubmissionVersion: 8,
		CreatedAt: now.Add(-time.Minute),
	}
}

type expiredJobTestValue struct {
	jobID        uuid.UUID
	submissionID uuid.UUID
	generation   int64
	phase        string
	attempts     uint32
	maxAttempts  uint32
	attemptID    uuid.UUID
	workerID     uuid.UUID
	workerBootID uuid.UUID
}

func expiredJobTestRow(
	sequence int,
	phase string,
	attempts uint32,
	maxAttempts uint32,
) expiredJobTestValue {
	return expiredJobTestValue{
		jobID:        testutil.ID(sequence*10 + 1),
		submissionID: testutil.ID(sequence*10 + 2),
		generation:   1, phase: phase, attempts: attempts, maxAttempts: maxAttempts,
		attemptID:    testutil.ID(sequence*10 + 3),
		workerID:     testutil.ID(sequence*10 + 4),
		workerBootID: testutil.ID(sequence*10 + 5),
	}
}

func (value expiredJobTestValue) values() []driver.Value {
	return []driver.Value{
		value.jobID, value.submissionID, value.generation, value.phase,
		value.attempts, value.maxAttempts, value.attemptID,
		value.workerID, value.workerBootID,
	}
}

func newJobSQLRepository(t *testing.T) (*JobRepository, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
	)
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repository, err := NewJobRepository(database, JobRepositoryConfig{
		HeartbeatFreshness: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewJobRepository() error = %v", err)
	}
	return repository, mock
}

func expectJobDatabaseNow(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(`SELECT clock_timestamp\(\)`).
		WillReturnRows(sqlmock.NewRows([]string{"clock_timestamp"}).AddRow(now))
}

func expectJobClaimAdvisoryLock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
}

func expectJobBacklogAdmissionAdvisoryLock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).
		WithArgs(submissioncontract.JudgeBacklogAdmissionAdvisoryLock).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
}

func expectCurrentClaimWorker(mock sqlmock.Sqlmock, now time.Time, capacity int) {
	mock.ExpectQuery(`SELECT worker\.claims_enabled.*NOT EXISTS.*FOR UPDATE OF worker`).
		WillReturnRows(sqlmock.NewRows([]string{
			"claims_enabled", "heartbeat_at", "free_capacity",
		}).AddRow(true, now, capacity))
}

func expectHeartbeatCurrentWorker(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(`SELECT worker\.heartbeat_at.*NOT EXISTS`).
		WillReturnRows(sqlmock.NewRows([]string{"heartbeat_at"}).AddRow(now))
}

func expectFencedLeaseWorker(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(`SELECT worker\.heartbeat_at.*NOT EXISTS`).
		WithArgs(testutil.ID(1), testutil.ID(2)).
		WillReturnRows(sqlmock.NewRows([]string{"heartbeat_at"}).AddRow(now))
}

func expectFencedLeaseRow(
	mock sqlmock.Sqlmock,
	lease ports.JobLease,
	now time.Time,
	phase string,
	attemptState string,
	version int64,
	attempts uint32,
	maxAttempts uint32,
) {
	hash := sha256.Sum256(lease.LeaseToken)
	mock.ExpectQuery(`SELECT job\.submission_id.*FOR UPDATE OF job, attempt, submission`).
		WithArgs(lease.Job.ID, lease.Attempt.ID).
		WillReturnRows(sqlmock.NewRows([]string{
			"submission_id", "generation", "judge_spec_id",
			"attempts", "max_attempts", "job_state", "lease_owner",
			"lease_token_hash", "lease_until", "worker_id", "worker_boot_id",
			"attempt_state", "phase", "submission_version",
		}).AddRow(
			lease.Job.SubmissionID,
			lease.Job.Generation,
			lease.Job.JudgeSpecID,
			attempts,
			maxAttempts,
			"leased",
			lease.LeaseOwner,
			hash[:],
			now.Add(time.Minute),
			lease.Attempt.WorkerID,
			lease.Attempt.WorkerBootID,
			attemptState,
			phase,
			version,
		))
}

func exactClaimQueryPattern() string {
	return `SELECT.*FROM judge_jobs AS j.*` +
		`capability\.probed_at >= evidence_worker\.boot_started_at.*` +
		`capability\.runtime_profile_version = s\.runtime_profile_version.*` +
		`capability\.compiler_version = s\.compiler_version.*` +
		`capability\.runtime_bootstrap_version = s\.runtime_bootstrap_version.*` +
		`capability\.compile_command_id = s\.compile_command_id.*` +
		`capability\.run_command_id = s\.run_command_id.*` +
		`capability\.image_digest = s\.sandbox_image_digest.*` +
		`capability\.manifest_checksum = s\.artifact_manifest_checksum.*` +
		`capability\.supervisor_protocol = s\.supervisor_protocol_version.*` +
		`capability\.checker_key = s\.checker_key.*` +
		`capability\.checker_version = s\.checker_version.*` +
		`capability\.capability_profile_hash = s\.capability_profile_hash.*` +
		`FOR UPDATE OF j, sub SKIP LOCKED`
}

func expectExpiredAttemptClosure(
	mock sqlmock.Sqlmock,
	value expiredJobTestValue,
	now time.Time,
) {
	mock.ExpectExec(`UPDATE judge_attempts\s+SET state = 'lease_lost'`).
		WithArgs(value.attemptID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectWorkerCapacityRelease(
	mock sqlmock.Sqlmock,
	value expiredJobTestValue,
	now time.Time,
) {
	mock.ExpectExec(`UPDATE judge_worker_instances\s+SET free_capacity = LEAST`).
		WithArgs(value.workerID, value.workerBootID, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
