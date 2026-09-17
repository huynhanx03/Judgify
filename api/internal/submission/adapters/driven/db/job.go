package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
	submissioncontract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

const (
	claimAdvisoryLock    int64 = 0x4a554447494659
	leaseTokenBytes            = 32
	maximumClaimBatch          = 64
	maximumReapBatch           = 256
	maximumLeaseDuration       = 24 * time.Hour
)

var ErrInvalidJobRepository = errors.New("submission jobs: invalid repository operation")

type jobSQLDatabase interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type JobRepositoryConfig struct {
	HeartbeatFreshness time.Duration
	LaneWeights        map[string]int
}

type JobRepository struct {
	db        jobSQLDatabase
	random    io.Reader
	newID     func() uuid.UUID
	freshness time.Duration

	laneMu      sync.Mutex
	laneCredit  [3]int
	laneWeights [3]int
}

func NewJobRepository(database jobSQLDatabase, config JobRepositoryConfig) (*JobRepository, error) {
	if database == nil || config.HeartbeatFreshness <= 0 {
		return nil, ErrInvalidJobRepository
	}
	weights := [3]int{60, 35, 5}
	knownLanes := map[string]struct{}{
		"contest": {}, "regular": {}, "maintenance": {},
	}
	for index, lane := range [...]string{"contest", "regular", "maintenance"} {
		if value, ok := config.LaneWeights[lane]; ok {
			if value <= 0 {
				return nil, ErrInvalidJobRepository
			}
			weights[index] = value
		}
	}
	for lane := range config.LaneWeights {
		if _, known := knownLanes[lane]; !known {
			return nil, ErrInvalidJobRepository
		}
	}
	totalWeight := 0
	for _, weight := range weights {
		if weight > int(^uint(0)>>1)-totalWeight {
			return nil, ErrInvalidJobRepository
		}
		totalWeight += weight
	}
	return &JobRepository{
		db: database, random: rand.Reader, newID: uuid.New,
		freshness: config.HeartbeatFreshness, laneWeights: weights,
	}, nil
}

// NewJobRepositoryFromEnt borrows the process SQL pool. The repository never
// closes the pool and never performs schema changes.
func NewJobRepositoryFromEnt(client *dbent.EntClient, config JobRepositoryConfig) (*JobRepository, error) {
	if client == nil {
		return nil, ErrInvalidJobRepository
	}
	driver, ok := client.DialectDriver()
	if !ok {
		return nil, ErrInvalidJobRepository
	}
	sqlDriver, ok := driver.(*entsql.Driver)
	if !ok || sqlDriver.DB() == nil {
		return nil, ErrInvalidJobRepository
	}
	return NewJobRepository(sqlDriver.DB(), config)
}

type claimCandidate struct {
	jobID             uuid.UUID
	submissionID      uuid.UUID
	generation        int64
	judgeSpecID       uuid.UUID
	lane              string
	basePriority      int
	jobCreatedAt      time.Time
	attempts          uint32
	maxAttempts       uint32
	originCID         string
	submissionVersion int64
	spec              ports.JudgeSpecClaim
}

// Claim locks the exact worker boot, serializes configured running bounds,
// matches one complete normalized capability tuple, and commits the job,
// attempt, and queued-to-claimed transition together.
func (repository *JobRepository) Claim(
	ctx context.Context,
	request ports.JobClaimRequest,
) ([]ports.JobLease, error) {
	if repository == nil || repository.db == nil || ctx == nil ||
		request.WorkerID == uuid.Nil || request.WorkerBootID == uuid.Nil ||
		request.LeaseOwner == "" || len(request.LeaseOwner) > 128 ||
		request.LeaseDuration <= 0 || request.LeaseDuration > maximumLeaseDuration ||
		request.Capacity <= 0 || request.GlobalRunningLimit <= 0 ||
		request.PerUserRunningLimit <= 0 {
		return nil, ErrInvalidJobRepository
	}
	freshness := repository.freshness
	if request.HeartbeatFreshness > 0 && request.HeartbeatFreshness < freshness {
		freshness = request.HeartbeatFreshness
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin job claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := takeClaimAdvisoryLock(ctx, tx); err != nil {
		return nil, err
	}
	var claimsEnabled bool
	var heartbeatAt time.Time
	var freeCapacity int
	err = tx.QueryRowContext(ctx, `
		SELECT worker.claims_enabled, worker.heartbeat_at, worker.free_capacity
		FROM judge_worker_instances AS worker
		WHERE worker.id = $1 AND worker.boot_id = $2
			AND NOT EXISTS (
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
			)
		FOR UPDATE OF worker`, request.WorkerID, request.WorkerBootID,
	).Scan(&claimsEnabled, &heartbeatAt, &freeCapacity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrClaimDenied
	}
	if err != nil {
		return nil, fmt.Errorf("lock worker claim boot: %w", err)
	}
	if !claimsEnabled || heartbeatAt.After(now) ||
		now.Sub(heartbeatAt) > freshness || freeCapacity <= 0 {
		return nil, ports.ErrClaimDenied
	}
	capacity := minInt(request.Capacity, freeCapacity, maximumClaimBatch)
	leases := make([]ports.JobLease, 0, capacity)
	committed := false
	defer func() {
		if !committed {
			wipeRepositoryLeaseTokens(leases)
		}
	}()
	for len(leases) < capacity {
		candidate, found, claimErr := repository.nextCompatibleCandidate(ctx, tx, request, now)
		if claimErr != nil {
			return nil, claimErr
		}
		if !found {
			break
		}
		lease, leaseErr := repository.leaseCandidate(ctx, tx, request, now, candidate)
		if leaseErr != nil {
			return nil, leaseErr
		}
		leases = append(leases, lease)
	}
	if len(leases) != 0 {
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE judge_worker_instances
			SET free_capacity = free_capacity - $3, updated_at = $4
			WHERE id = $1 AND boot_id = $2 AND free_capacity >= $3`,
			request.WorkerID, request.WorkerBootID, len(leases), now,
		)
		if updateErr != nil {
			return nil, fmt.Errorf("reserve worker capacity: %w", updateErr)
		}
		if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
			return nil, ports.ErrClaimDenied
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit job claim: %w", err)
	}
	committed = true
	return leases, nil
}

func (repository *JobRepository) nextCompatibleCandidate(
	ctx context.Context,
	tx *sql.Tx,
	request ports.JobClaimRequest,
	now time.Time,
) (claimCandidate, bool, error) {
	preferred := repository.nextLane()
	for offset := 0; offset < 3; offset++ {
		lane := [...]string{"contest", "regular", "maintenance"}[(preferred+offset)%3]
		candidate, found, err := queryClaimCandidate(ctx, tx, request, now, lane)
		if err != nil {
			return claimCandidate{}, false, err
		}
		if found {
			return candidate, true, nil
		}
	}
	return claimCandidate{}, false, nil
}

func (repository *JobRepository) nextLane() int {
	repository.laneMu.Lock()
	defer repository.laneMu.Unlock()
	total := 0
	selected := 0
	for index, weight := range repository.laneWeights {
		repository.laneCredit[index] += weight
		total += weight
		if repository.laneCredit[index] > repository.laneCredit[selected] {
			selected = index
		}
	}
	repository.laneCredit[selected] -= total
	return selected
}

const exactClaimCandidateSQL = `
	SELECT
		j.id, j.submission_id, j.generation, j.judge_spec_id, j.lane,
		j.base_priority, j.created_at, j.attempts, j.max_attempts, j.origin_cid,
		sub.version,
		s.id, s.submission_id, s.generation, s.user_id,
			s.problem_revision_id, s.testset_revision_id, s.judge_spec_protocol,
			s.runtime_key, s.runtime_profile_version, s.compiler_version,
			s.runtime_bootstrap_version, s.compile_command_id, s.run_command_id,
			s.sandbox_image_digest,
		s.artifact_manifest_version, s.artifact_manifest_checksum,
			s.supervisor_protocol_version, s.checker_key, s.checker_version,
			s.release_id, s.approved_release_manifest_checksum,
			s.checker_implementation_checksum, s.capability_profile_hash,
			s.spec_checksum, s.reward_snapshot_checksum
	FROM judge_jobs AS j
	JOIN judge_specs AS s
		ON s.id = j.judge_spec_id
		AND s.submission_id = j.submission_id
		AND s.generation = j.generation
	JOIN submissions AS sub
		ON sub.id = j.submission_id
		AND sub.latest_generation = j.generation
		AND sub.latest_generation_phase = 'queued'
		AND sub.latest_generation_outcome IS NULL
	WHERE j.state = 'ready'
		AND j.available_at <= $1
		AND j.attempts < j.max_attempts
		AND j.lane = $2
		AND EXISTS (
			SELECT 1
			FROM judge_worker_capabilities AS capability
				JOIN judge_worker_instances AS evidence_worker
					ON evidence_worker.id = capability.worker_instance_id
					AND evidence_worker.boot_id = capability.boot_id
				WHERE capability.worker_instance_id = $3
					AND capability.boot_id = $4
					AND capability.passing
					AND capability.probed_at >= evidence_worker.boot_started_at
					AND capability.probed_at <= $1
				AND capability.judge_spec_protocol = s.judge_spec_protocol
					AND capability.runtime_key = s.runtime_key
					AND capability.runtime_profile_version = s.runtime_profile_version
					AND capability.compiler_version = s.compiler_version
					AND capability.runtime_bootstrap_version = s.runtime_bootstrap_version
					AND capability.compile_command_id = s.compile_command_id
					AND capability.run_command_id = s.run_command_id
					AND capability.image_digest = s.sandbox_image_digest
				AND capability.manifest_version = s.artifact_manifest_version
				AND capability.manifest_checksum = s.artifact_manifest_checksum
				AND capability.supervisor_protocol = s.supervisor_protocol_version
				AND capability.checker_key = s.checker_key
				AND capability.checker_version = s.checker_version
				AND capability.checker_implementation_checksum = s.checker_implementation_checksum
				AND capability.capability_profile_hash = s.capability_profile_hash
		)
		AND EXISTS (
			SELECT 1
			FROM activated_judge_capabilities AS activation
			WHERE activation.release_id = s.release_id
				AND activation.approved_release_manifest_checksum = s.approved_release_manifest_checksum
				AND activation.state = 'active'
				AND activation.retired_at IS NULL
				AND activation.judge_spec_protocol = s.judge_spec_protocol
				AND activation.runtime_key = s.runtime_key
				AND activation.runtime_profile_version = s.runtime_profile_version
				AND activation.compiler_version = s.compiler_version
				AND activation.runtime_bootstrap_version = s.runtime_bootstrap_version
				AND activation.compile_command_id = s.compile_command_id
				AND activation.run_command_id = s.run_command_id
				AND activation.image_digest = s.sandbox_image_digest
				AND activation.manifest_version = s.artifact_manifest_version
				AND activation.manifest_checksum = s.artifact_manifest_checksum
				AND activation.supervisor_protocol = s.supervisor_protocol_version
				AND activation.checker_key = s.checker_key
				AND activation.checker_version = s.checker_version
				AND activation.checker_implementation_checksum = s.checker_implementation_checksum
				AND activation.capability_profile_hash = s.capability_profile_hash
		)
		AND (SELECT count(*) FROM judge_jobs AS running WHERE running.state = 'leased') < $5
		AND (
			SELECT count(*)
			FROM judge_jobs AS user_job
			JOIN judge_specs AS user_spec ON user_spec.id = user_job.judge_spec_id
			WHERE user_job.state = 'leased' AND user_spec.user_id = s.user_id
		) < $6
	ORDER BY
		j.base_priority::bigint
			+ GREATEST(0, floor(extract(epoch FROM ($1 - j.created_at)) / 60))::bigint DESC,
		j.created_at ASC,
		j.id ASC
	FOR UPDATE OF j, sub SKIP LOCKED
	LIMIT 1`

func queryClaimCandidate(
	ctx context.Context,
	tx *sql.Tx,
	request ports.JobClaimRequest,
	now time.Time,
	lane string,
) (claimCandidate, bool, error) {
	var value claimCandidate
	value.spec = ports.JudgeSpecClaim{}
	err := tx.QueryRowContext(
		ctx, exactClaimCandidateSQL,
		now, lane, request.WorkerID, request.WorkerBootID,
		request.GlobalRunningLimit, request.PerUserRunningLimit,
	).Scan(
		&value.jobID, &value.submissionID, &value.generation, &value.judgeSpecID, &value.lane,
		&value.basePriority, &value.jobCreatedAt, &value.attempts, &value.maxAttempts,
		&value.originCID, &value.submissionVersion,
		&value.spec.ID, &value.spec.SubmissionID, &value.spec.Generation, &value.spec.UserID,
		&value.spec.ProblemRevisionID, &value.spec.TestsetRevisionID,
		&value.spec.JudgeSpecProtocol, &value.spec.RuntimeKey,
		&value.spec.RuntimeProfileVersion, &value.spec.CompilerVersion,
		&value.spec.RuntimeBootstrapVersion, &value.spec.CompileCommandID,
		&value.spec.RunCommandID, &value.spec.SandboxImageDigest,
		&value.spec.ArtifactManifestVersion, &value.spec.ArtifactManifestChecksum,
		&value.spec.SupervisorProtocol, &value.spec.CheckerKey, &value.spec.CheckerVersion,
		&value.spec.ReleaseID, &value.spec.ApprovedManifestChecksum,
		&value.spec.CheckerImplementationChecksum, &value.spec.CapabilityProfileHash, &value.spec.SpecChecksum,
		&value.spec.RewardSnapshotChecksum,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return claimCandidate{}, false, nil
	}
	if err != nil {
		return claimCandidate{}, false, fmt.Errorf("select compatible judge job: %w", err)
	}
	return value, true, nil
}

func (repository *JobRepository) leaseCandidate(
	ctx context.Context,
	tx *sql.Tx,
	request ports.JobClaimRequest,
	now time.Time,
	candidate claimCandidate,
) (ports.JobLease, error) {
	token := make([]byte, leaseTokenBytes)
	transferred := false
	defer func() {
		if !transferred {
			wipeBytes(token)
		}
	}()
	if _, err := io.ReadFull(repository.random, token); err != nil {
		return ports.JobLease{}, fmt.Errorf("create lease capability: %w", err)
	}
	hash := sha256.Sum256(token)
	if subtle.ConstantTimeCompare(hash[:], make([]byte, leaseTokenBytes)) == 1 {
		return ports.JobLease{}, ErrInvalidJobRepository
	}
	leaseUntil := now.Add(request.LeaseDuration)
	attemptNumber := candidate.attempts + 1
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'leased', attempts = attempts + 1,
			lease_owner = $2, lease_token_hash = $3, lease_until = $4,
			heartbeat_at = $5, started_at = COALESCE(started_at, $5), updated_at = $5
		WHERE id = $1 AND state = 'ready' AND attempts = $6 AND attempts < max_attempts`,
		candidate.jobID, request.LeaseOwner, hash[:], leaseUntil, now, candidate.attempts,
	)
	if err != nil {
		return ports.JobLease{}, fmt.Errorf("lease compatible judge job: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.JobLease{}, ports.ErrLeaseLost
	}
	attemptID := repository.newID()
	if attemptID == uuid.Nil {
		return ports.JobLease{}, ErrInvalidJobRepository
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO judge_attempts (
			id, created_at, updated_at, job_id, submission_id, generation, judge_spec_id,
			attempt_number, lease_owner, lease_token_hash, worker_id, worker_boot_id,
			origin_cid, state, started_at, heartbeat_at,
			runtime_profile_version, compiler_version, runtime_bootstrap_version,
			supervisor_protocol_version, sandbox_image_digest, artifact_manifest_checksum
		) VALUES (
			$1, $2, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			$12, 'created', $2, $2, $13, $14, $15, $16, $17, $18
		)`,
		attemptID, now, candidate.jobID, candidate.submissionID, candidate.generation,
		candidate.judgeSpecID, attemptNumber, request.LeaseOwner, hash[:], request.WorkerID,
		request.WorkerBootID, candidate.originCID, candidate.spec.RuntimeProfileVersion,
		candidate.spec.CompilerVersion, candidate.spec.RuntimeBootstrapVersion,
		candidate.spec.SupervisorProtocol, candidate.spec.SandboxImageDigest,
		candidate.spec.ArtifactManifestChecksum,
	)
	if err != nil {
		return ports.JobLease{}, fmt.Errorf("insert fenced judge attempt: %w", err)
	}
	var nextVersion int64
	err = tx.QueryRowContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = 'claimed',
			latest_generation_started_at = COALESCE(latest_generation_started_at, $4),
			version = version + 1, updated_at = $4
		WHERE id = $1 AND latest_generation = $2 AND version = $3
			AND latest_generation_phase = 'queued'
			AND latest_generation_outcome IS NULL
		RETURNING version`, candidate.submissionID, candidate.generation,
		candidate.submissionVersion, now,
	).Scan(&nextVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.JobLease{}, ports.ErrLeaseLost
	}
	if err != nil {
		return ports.JobLease{}, fmt.Errorf("claim submission generation: %w", err)
	}
	leaseHash := entity.LeaseTokenHash(hash)
	lease := ports.JobLease{
		Job: entity.JudgeJob{
			ID: candidate.jobID, SubmissionID: candidate.submissionID,
			Generation: candidate.generation, JudgeSpecID: candidate.judgeSpecID,
			Lane: candidate.lane, State: entity.JobLeased,
			Attempts: attemptNumber, MaxAttempts: candidate.maxAttempts,
			OriginCID: candidate.originCID,
		},
		Attempt: entity.JudgeAttempt{
			ID: attemptID, JobID: candidate.jobID, SubmissionID: candidate.submissionID,
			Generation: candidate.generation, JudgeSpecID: candidate.judgeSpecID,
			AttemptNumber: attemptNumber, LeaseTokenHash: leaseHash,
			WorkerID: request.WorkerID, WorkerBootID: request.WorkerBootID,
			State: entity.AttemptCreated, StartedAt: now,
		},
		JudgeSpec: candidate.spec, LeaseOwner: request.LeaseOwner,
		LeaseToken: token, LeaseUntil: leaseUntil,
		SubmissionVersion: nextVersion, BasePriority: candidate.basePriority,
		CreatedAt: candidate.jobCreatedAt,
	}
	transferred = true
	return lease, nil
}

// HeartbeatWorkerBoot advances liveness only for the current persisted boot.
// It never enables claims or rewrites capacity/evidence.
func (repository *JobRepository) HeartbeatWorkerBoot(
	ctx context.Context,
	heartbeat ports.WorkerBootHeartbeat,
) error {
	if repository == nil || repository.db == nil || ctx == nil ||
		heartbeat.WorkerID == uuid.Nil || heartbeat.WorkerBootID == uuid.Nil {
		return ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin worker boot heartbeat: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return err
	}
	if err := takeClaimAdvisoryLock(ctx, tx); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_worker_instances AS worker
		SET heartbeat_at = $3,
			free_capacity = GREATEST(
				0,
				worker.total_capacity - (
					SELECT count(*)::integer
					FROM judge_jobs AS job
					JOIN judge_attempts AS attempt
						ON attempt.job_id = job.id
						AND attempt.attempt_number = job.attempts
					WHERE job.state = 'leased'
						AND attempt.worker_id = worker.id
						AND attempt.worker_boot_id = worker.boot_id
				)
			),
			updated_at = $3
		WHERE worker.id = $1 AND worker.boot_id = $2
			AND worker.boot_started_at <= $3
			AND worker.heartbeat_at <= $3
			AND NOT EXISTS (
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
			)`,
		heartbeat.WorkerID, heartbeat.WorkerBootID, now,
	)
	if err != nil {
		return fmt.Errorf("heartbeat current worker boot: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrWorkerBootLost
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit worker boot heartbeat: %w", err)
	}
	return nil
}

// Heartbeat compares the fetched secret hash in constant time, then fences
// both job and attempt by owner, worker boot, current generation, and expiry.
func (repository *JobRepository) Heartbeat(
	ctx context.Context,
	heartbeat ports.LeaseHeartbeat,
) (time.Time, error) {
	if repository == nil || repository.db == nil || ctx == nil ||
		heartbeat.JobID == uuid.Nil || heartbeat.AttemptID == uuid.Nil ||
		heartbeat.WorkerID == uuid.Nil || heartbeat.WorkerBootID == uuid.Nil ||
		heartbeat.LeaseOwner == "" || len(heartbeat.LeaseOwner) > 128 ||
		len(heartbeat.LeaseToken) != leaseTokenBytes ||
		heartbeat.ExtendTo.IsZero() {
		return time.Time{}, ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return time.Time{}, fmt.Errorf("begin job heartbeat: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return time.Time{}, err
	}
	if !heartbeat.ExtendTo.After(now) || heartbeat.ExtendTo.After(now.Add(maximumLeaseDuration)) {
		return time.Time{}, ErrInvalidJobRepository
	}
	var workerHeartbeat time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT worker.heartbeat_at
		FROM judge_worker_instances AS worker
		WHERE worker.id = $1 AND worker.boot_id = $2
			AND NOT EXISTS (
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
			)
		`,
		heartbeat.WorkerID, heartbeat.WorkerBootID,
	).Scan(&workerHeartbeat)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ports.ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("validate heartbeat worker boot: %w", err)
	}
	if workerHeartbeat.After(now) ||
		now.Sub(workerHeartbeat) > repository.freshness {
		return time.Time{}, ports.ErrLeaseLost
	}
	var owner string
	var storedHash []byte
	var leaseUntil time.Time
	var submissionID uuid.UUID
	var generation int64
	var state string
	err = tx.QueryRowContext(ctx, `
		SELECT lease_owner, lease_token_hash, lease_until, submission_id, generation, state
		FROM judge_jobs WHERE id = $1 FOR UPDATE`, heartbeat.JobID,
	).Scan(&owner, &storedHash, &leaseUntil, &submissionID, &generation, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ports.ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("lock heartbeat job: %w", err)
	}
	presentedHash := sha256.Sum256(heartbeat.LeaseToken)
	normalizedStoredHash := make([]byte, sha256.Size)
	copy(normalizedStoredHash, storedHash)
	hashMatches := subtle.ConstantTimeEq(int32(len(storedHash)), sha256.Size) &
		subtle.ConstantTimeCompare(normalizedStoredHash, presentedHash[:])
	ownerMatches := constantTimeTextEqual(owner, heartbeat.LeaseOwner)
	if state != string(entity.JobLeased) || hashMatches != 1 ||
		!ownerMatches || !leaseUntil.After(now) {
		return time.Time{}, ports.ErrLeaseLost
	}
	var (
		attemptOwner  string
		attemptHash   []byte
		attemptWorker uuid.UUID
		attemptBoot   uuid.UUID
		attemptState  string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT lease_owner, lease_token_hash, worker_id, worker_boot_id, state
		FROM judge_attempts
		WHERE id = $1 AND job_id = $2 AND submission_id = $3 AND generation = $4
		FOR UPDATE`,
		heartbeat.AttemptID, heartbeat.JobID, submissionID, generation,
	).Scan(
		&attemptOwner, &attemptHash, &attemptWorker, &attemptBoot, &attemptState,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ports.ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("lock heartbeat attempt: %w", err)
	}
	normalizedAttemptHash := make([]byte, sha256.Size)
	copy(normalizedAttemptHash, attemptHash)
	attemptHashMatches := subtle.ConstantTimeEq(int32(len(attemptHash)), sha256.Size) &
		subtle.ConstantTimeCompare(normalizedAttemptHash, presentedHash[:])
	if attemptHashMatches != 1 ||
		!constantTimeTextEqual(attemptOwner, heartbeat.LeaseOwner) ||
		attemptWorker != heartbeat.WorkerID ||
		attemptBoot != heartbeat.WorkerBootID ||
		!persistedAttemptInFlight(attemptState) {
		return time.Time{}, ports.ErrLeaseLost
	}
	var currentGeneration int64
	var phase string
	err = tx.QueryRowContext(ctx, `
		SELECT latest_generation, latest_generation_phase
		FROM submissions WHERE id = $1 FOR SHARE`, submissionID,
	).Scan(&currentGeneration, &phase)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ports.ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("validate heartbeat generation: %w", err)
	}
	if currentGeneration != generation {
		return time.Time{}, ports.ErrLeaseLost
	}
	if phase == string(entity.PhaseCancelled) || phase == string(entity.PhaseTerminal) {
		return time.Time{}, ports.ErrLeaseCancelled
	}
	if phase != string(entity.PhaseClaimed) && phase != string(entity.PhaseCompiling) &&
		phase != string(entity.PhaseRunning) && phase != string(entity.PhaseFinalizing) {
		return time.Time{}, ports.ErrLeaseLost
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET heartbeat_at = $7, updated_at = $7
		WHERE id = $1 AND job_id = $2 AND worker_id = $3 AND worker_boot_id = $4
			AND lease_owner = $5 AND lease_token_hash = $6
			AND state IN ('created','preparing','compiling','running','finalizing')`,
		heartbeat.AttemptID, heartbeat.JobID, heartbeat.WorkerID, heartbeat.WorkerBootID,
		heartbeat.LeaseOwner, presentedHash[:], now,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf("heartbeat judge attempt: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return time.Time{}, ports.ErrLeaseLost
	}
	var acceptedLeaseUntil time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE judge_jobs SET lease_until = $2, heartbeat_at = $3, updated_at = $3
		WHERE id = $1 AND state = 'leased' AND lease_until > $3
		RETURNING lease_until`,
		heartbeat.JobID, heartbeat.ExtendTo.UTC(), now,
	).Scan(&acceptedLeaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ports.ErrLeaseLost
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("heartbeat judge job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return time.Time{}, fmt.Errorf("commit job heartbeat: %w", err)
	}
	return acceptedLeaseUntil.UTC(), nil
}

type expiredJob struct {
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

func (repository *JobRepository) ReapExpired(
	ctx context.Context,
	request ports.ReapRequest,
) (ports.ReapResult, error) {
	if repository == nil || repository.db == nil || ctx == nil || request.Limit <= 0 ||
		request.Limit > maximumReapBatch || request.RetryInitial <= 0 ||
		request.RetryMaximum < request.RetryInitial ||
		request.RetryMaximum > maximumLeaseDuration {
		return ports.ReapResult{}, ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return ports.ReapResult{}, fmt.Errorf("begin expired lease reaper: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return ports.ReapResult{}, err
	}
	if err := takeClaimAdvisoryLock(ctx, tx); err != nil {
		return ports.ReapResult{}, err
	}
	// A retry increases the ready/retry_wait backlog observed by intake.
	// Serialize that possible increase before taking any job row lock so
	// admission cannot race an expired lease back into the bounded backlog.
	if err := takeBacklogAdmissionAdvisoryLock(ctx, tx); err != nil {
		return ports.ReapResult{}, err
	}
	result := ports.ReapResult{}
	promoted, err := tx.ExecContext(ctx, `
		WITH promotable AS (
			SELECT job.id
			FROM judge_jobs AS job
			WHERE job.state = 'retry_wait' AND job.available_at <= $1
				AND job.attempts < job.max_attempts
				AND EXISTS (
					SELECT 1
					FROM submissions AS submission
					WHERE submission.id = job.submission_id
						AND submission.latest_generation = job.generation
						AND submission.latest_generation_phase = 'queued'
						AND submission.latest_generation_outcome IS NULL
				)
			ORDER BY job.available_at ASC, job.id ASC
			FOR UPDATE OF job SKIP LOCKED
			LIMIT $2
		)
		UPDATE judge_jobs AS job
		SET state = 'ready', updated_at = $1
		FROM promotable
		WHERE job.id = promotable.id`, now, request.Limit)
	if err != nil {
		return ports.ReapResult{}, fmt.Errorf("promote retry jobs: %w", err)
	}
	if affected, affectErr := promoted.RowsAffected(); affectErr != nil {
		return ports.ReapResult{}, fmt.Errorf("count promoted retry jobs: %w", affectErr)
	} else {
		result.Promoted = int(affected)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT j.id, j.submission_id, j.generation,
			COALESCE(submission.latest_generation_phase, 'cancelled'),
			j.attempts, j.max_attempts,
			a.id, a.worker_id, a.worker_boot_id
		FROM judge_jobs AS j
		JOIN judge_attempts AS a ON a.job_id = j.id AND a.attempt_number = j.attempts
		LEFT JOIN submissions AS submission
			ON submission.id = j.submission_id
			AND submission.latest_generation = j.generation
		WHERE j.state = 'leased' AND j.lease_until <= $1
		ORDER BY j.lease_until ASC, j.id ASC
		FOR UPDATE OF j, a SKIP LOCKED
		LIMIT $2`, now, request.Limit)
	if err != nil {
		return ports.ReapResult{}, fmt.Errorf("select expired leases: %w", err)
	}
	values := make([]expiredJob, 0, request.Limit)
	for rows.Next() {
		var value expiredJob
		if err := rows.Scan(
			&value.jobID, &value.submissionID, &value.generation, &value.phase,
			&value.attempts, &value.maxAttempts, &value.attemptID,
			&value.workerID, &value.workerBootID,
		); err != nil {
			_ = rows.Close()
			return ports.ReapResult{}, fmt.Errorf("scan expired lease: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Close(); err != nil {
		return ports.ReapResult{}, fmt.Errorf("close expired lease rows: %w", err)
	}
	for _, value := range values {
		if value.phase == string(entity.PhaseCancelled) ||
			value.phase == string(entity.PhaseTerminal) {
			if err := cancelExpiredJob(ctx, tx, value, now); err != nil {
				return ports.ReapResult{}, err
			}
			result.Cancelled++
			if err := releaseWorkerCapacity(ctx, tx, value, now); err != nil {
				return ports.ReapResult{}, err
			}
			continue
		}
		attemptResult, err := tx.ExecContext(ctx, `
			UPDATE judge_attempts
			SET state = 'lease_lost', failure_classification = 'infrastructure',
				failure_code = 'lease_expired', failure_summary = 'worker lease expired',
				finished_at = $2, updated_at = $2
			WHERE id = $1 AND state IN ('created','preparing','compiling','running','finalizing')`,
			value.attemptID, now,
		)
		if err != nil {
			return ports.ReapResult{}, fmt.Errorf("close expired attempt: %w", err)
		}
		if affected, affectErr := attemptResult.RowsAffected(); affectErr != nil || affected != 1 {
			return ports.ReapResult{}, ports.ErrLeaseLost
		}
		if value.attempts >= value.maxAttempts {
			if err := repository.deadExpiredJob(ctx, tx, value, now); err != nil {
				return ports.ReapResult{}, err
			}
			result.Dead++
		} else {
			delay, err := repository.fullJitterBackoff(value.attempts, request.RetryInitial, request.RetryMaximum)
			if err != nil {
				return ports.ReapResult{}, err
			}
			if err := retryExpiredJob(ctx, tx, value, now, now.Add(delay)); err != nil {
				return ports.ReapResult{}, err
			}
			result.Retried++
		}
		if err := releaseWorkerCapacity(ctx, tx, value, now); err != nil {
			return ports.ReapResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ports.ReapResult{}, fmt.Errorf("commit expired lease reaper: %w", err)
	}
	return result, nil
}

// AcknowledgeCancellation closes an active lease only after the aggregate has
// been deliberately cancelled. The worker calls it after sandbox cleanup;
// keeping this separate from cancellation command handling gives the running
// process a bounded chance to stop while the lease capability remains fenced.
func (repository *JobRepository) AcknowledgeCancellation(
	ctx context.Context,
	lease ports.JobLease,
) error {
	if repository == nil || repository.db == nil || ctx == nil ||
		!validJobLeaseCapability(lease) {
		return ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin cancelled lease acknowledgement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return err
	}
	var (
		submissionID     uuid.UUID
		generation       int64
		judgeSpecID      uuid.UUID
		jobState         string
		owner            sql.NullString
		jobTokenHash     []byte
		attemptTokenHash []byte
		workerID         uuid.UUID
		workerBootID     uuid.UUID
		attemptState     string
		phase            string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT job.submission_id, job.generation, job.judge_spec_id,
			job.state, job.lease_owner, job.lease_token_hash,
			attempt.lease_token_hash, attempt.worker_id, attempt.worker_boot_id,
			attempt.state, submission.latest_generation_phase
		FROM judge_jobs AS job
		JOIN judge_attempts AS attempt
			ON attempt.id = $2
			AND attempt.job_id = job.id
			AND attempt.submission_id = job.submission_id
			AND attempt.generation = job.generation
			AND attempt.judge_spec_id = job.judge_spec_id
		JOIN submissions AS submission
			ON submission.id = job.submission_id
			AND submission.latest_generation = job.generation
		WHERE job.id = $1
		FOR UPDATE OF job, attempt, submission`,
		lease.Job.ID, lease.Attempt.ID,
	).Scan(
		&submissionID, &generation, &judgeSpecID, &jobState, &owner,
		&jobTokenHash, &attemptTokenHash, &workerID, &workerBootID,
		&attemptState, &phase,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrLeaseLost
	}
	if err != nil {
		return fmt.Errorf("lock cancelled judge lease: %w", err)
	}
	presentedHash := sha256.Sum256(lease.LeaseToken)
	if !matchingLeaseTokenHash(attemptTokenHash, presentedHash) ||
		subtle.ConstantTimeCompare(lease.Attempt.LeaseTokenHash[:], presentedHash[:]) != 1 ||
		submissionID != lease.Job.SubmissionID || generation != lease.Job.Generation ||
		judgeSpecID != lease.Job.JudgeSpecID || workerID != lease.Attempt.WorkerID ||
		workerBootID != lease.Attempt.WorkerBootID || phase != string(entity.PhaseCancelled) {
		return ports.ErrLeaseLost
	}
	// A replay after the first committed acknowledgement is intentionally a
	// no-op. The job transition is the one-time capacity-release boundary.
	if jobState == string(entity.JobCancelled) &&
		attemptState == string(entity.AttemptCancelled) {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit cancelled lease acknowledgement replay: %w", err)
		}
		return nil
	}
	if jobState != string(entity.JobLeased) || !matchingLeaseTokenHash(jobTokenHash, presentedHash) ||
		!constantTimeTextEqual(owner.String, lease.LeaseOwner) || !persistedAttemptInFlight(attemptState) {
		return ports.ErrLeaseLost
	}
	attemptResult, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET state = 'cancelled', failure_classification = 'cancellation',
			failure_code = 'generation_cancelled',
			failure_summary = 'submission generation was cancelled by an authorized actor',
			finished_at = $2, heartbeat_at = $2, updated_at = $2
		WHERE id = $1
			AND state IN ('created','preparing','compiling','running','finalizing')`,
		lease.Attempt.ID, now,
	)
	if err != nil {
		return fmt.Errorf("close cancelled judge attempt: %w", err)
	}
	if affected, affectErr := attemptResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	jobResult, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'cancelled', cancelled_at = $2,
			lease_owner = NULL, lease_token_hash = NULL,
			lease_until = NULL, heartbeat_at = NULL, updated_at = $2
		WHERE id = $1 AND state = 'leased'`,
		lease.Job.ID, now,
	)
	if err != nil {
		return fmt.Errorf("close cancelled judge job: %w", err)
	}
	if affected, affectErr := jobResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	if err := releaseWorkerCapacity(ctx, tx, expiredJob{
		jobID: lease.Job.ID, submissionID: submissionID, generation: generation,
		attemptID: lease.Attempt.ID, workerID: workerID, workerBootID: workerBootID,
	}, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cancelled lease acknowledgement: %w", err)
	}
	return nil
}

func cancelExpiredJob(
	ctx context.Context,
	tx *sql.Tx,
	value expiredJob,
	now time.Time,
) error {
	attemptResult, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET state = 'cancelled', failure_classification = 'cancellation',
			failure_code = 'generation_closed',
			failure_summary = 'submission generation is no longer executable',
			finished_at = $2, updated_at = $2
		WHERE id = $1
			AND state IN ('created','preparing','compiling','running','finalizing')`,
		value.attemptID, now,
	)
	if err != nil {
		return fmt.Errorf("cancel obsolete expired attempt: %w", err)
	}
	if affected, affectErr := attemptResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	jobResult, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'cancelled', cancelled_at = $2,
			lease_owner = NULL, lease_token_hash = NULL,
			lease_until = NULL, heartbeat_at = NULL, updated_at = $2
		WHERE id = $1 AND state = 'leased'`,
		value.jobID, now,
	)
	if err != nil {
		return fmt.Errorf("cancel obsolete expired job: %w", err)
	}
	if affected, affectErr := jobResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

func releaseWorkerCapacity(
	ctx context.Context,
	tx *sql.Tx,
	value expiredJob,
	now time.Time,
) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE judge_worker_instances
		SET free_capacity = LEAST(total_capacity, free_capacity + 1), updated_at = $3
		WHERE id = $1 AND boot_id = $2`,
		value.workerID, value.workerBootID, now,
	)
	if err != nil {
		return fmt.Errorf("release expired worker capacity: %w", err)
	}
	if affected, affectErr := result.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrWorkerBootLost
	}
	return nil
}

func retryExpiredJob(
	ctx context.Context,
	tx *sql.Tx,
	value expiredJob,
	now time.Time,
	retryAt time.Time,
) error {
	jobResult, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'retry_wait', available_at = $2,
			lease_owner = NULL, lease_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL,
			last_error_code = 'lease_expired', last_error_summary = 'worker lease expired', updated_at = $3
		WHERE id = $1 AND state = 'leased'`, value.jobID, retryAt, now)
	if err != nil {
		return fmt.Errorf("schedule expired job retry: %w", err)
	}
	if affected, affectErr := jobResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	submissionResult, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = 'queued', version = version + 1, updated_at = $3
		WHERE id = $1 AND latest_generation = $2
			AND latest_generation_phase IN ('claimed','compiling','running','finalizing')
			AND latest_generation_outcome IS NULL`, value.submissionID, value.generation, now)
	if err != nil {
		return fmt.Errorf("return expired generation to queue: %w", err)
	}
	if affected, affectErr := submissionResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	return nil
}

// deadExpiredJob closes an exhausted generation and writes its externally
// observable history in the same raw transaction as the attempt and capacity
// mutation performed by its caller. The state guard on judge_jobs provides the
// fencing/idempotency boundary for both the reaper and explicit retry path.
func (repository *JobRepository) deadExpiredJob(
	ctx context.Context,
	tx *sql.Tx,
	value expiredJob,
	now time.Time,
) error {
	if repository == nil || repository.newID == nil || tx == nil {
		return ErrInvalidJobRepository
	}
	judgementID := repository.newID()
	eventID := repository.newID()
	if judgementID == uuid.Nil || eventID == uuid.Nil {
		return ErrInvalidJobRepository
	}
	var facts struct {
		originCID         string
		userID            uuid.UUID
		problemID         uuid.UUID
		problemRevisionID uuid.UUID
		testsetRevisionID uuid.UUID
		submittedAt       time.Time
		version           int64
		phase             string
		outcome           sql.NullString
		activeJudgementID uuid.NullUUID
		activeGeneration  sql.NullInt64
		activeVerdict     sql.NullString
	}
	err := tx.QueryRowContext(ctx, `
		SELECT job.origin_cid, spec.user_id, spec.problem_id,
			spec.problem_revision_id, spec.testset_revision_id,
			submission.submitted_at, submission.version,
			submission.latest_generation_phase, submission.latest_generation_outcome,
			submission.active_judgement_id, submission.active_judgement_generation,
			submission.active_verdict
		FROM judge_jobs AS job
		JOIN judge_specs AS spec ON spec.id = job.judge_spec_id
		JOIN submissions AS submission ON submission.id = job.submission_id
		WHERE job.id = $1 AND job.submission_id = $2 AND job.generation = $3
		-- judge_specs is append-only; locking it is neither permitted nor needed.
		-- submissions is mutable and must be locked to observe a stable phase/version.
		FOR SHARE OF submission`, value.jobID, value.submissionID, value.generation).Scan(
		&facts.originCID, &facts.userID, &facts.problemID,
		&facts.problemRevisionID, &facts.testsetRevisionID,
		&facts.submittedAt, &facts.version, &facts.phase, &facts.outcome,
		&facts.activeJudgementID, &facts.activeGeneration, &facts.activeVerdict,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrLeaseLost
	}
	if err != nil || facts.originCID == "" || facts.userID == uuid.Nil || facts.problemID == uuid.Nil {
		return ErrInvalidJobRepository
	}
	jobResult, err := tx.ExecContext(ctx, `
		UPDATE judge_jobs
		SET state = 'dead', dead_at = $2,
			lease_owner = NULL, lease_token_hash = NULL, lease_until = NULL, heartbeat_at = NULL,
			last_error_code = 'attempts_exhausted',
			last_error_summary = 'judge infrastructure attempts exhausted', updated_at = $2
		WHERE id = $1 AND state = 'leased'`, value.jobID, now)
	if err != nil {
		return fmt.Errorf("mark expired job dead: %w", err)
	}
	if affected, affectErr := jobResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	judgementResult, err := tx.ExecContext(ctx, `
		INSERT INTO submission_judgements (
			id, created_at, submission_id, generation, judge_spec_id,
			problem_revision_id, testset_revision_id, verdict, passed_count, total_count,
			judge_spec_checksum, reward_snapshot, reward_snapshot_checksum,
			runtime_profile_version, compiler_version, runtime_bootstrap_version,
			supervisor_protocol_version, sandbox_image_digest, artifact_manifest_checksum,
			release_id, approved_release_manifest_checksum,
			checker_implementation_checksum, completed_at, origin_cid
		)
		SELECT $1, $2, spec.submission_id, spec.generation, spec.id,
			spec.problem_revision_id, spec.testset_revision_id, 'system_error', 0, 0,
			spec.spec_checksum, spec.reward_snapshot, spec.reward_snapshot_checksum,
			spec.runtime_profile_version, spec.compiler_version, spec.runtime_bootstrap_version,
			spec.supervisor_protocol_version, spec.sandbox_image_digest,
			spec.artifact_manifest_checksum, spec.release_id,
			spec.approved_release_manifest_checksum, spec.checker_implementation_checksum,
			$2, job.origin_cid
		FROM judge_jobs AS job
		JOIN judge_specs AS spec ON spec.id = job.judge_spec_id
		WHERE job.id = $3
		ON CONFLICT (submission_id, generation) DO NOTHING`,
		judgementID, now, value.jobID)
	if err != nil {
		return fmt.Errorf("record dead generation judgement: %w", err)
	}
	if affected, affectErr := judgementResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	submissionResult, err := tx.ExecContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = 'terminal', latest_generation_outcome = 'system_error',
			latest_generation_completed_at = $3,
			status = 'internal_error', passed_count = 0, total_count = 0,
			time_ms = NULL, memory_kb = NULL, error_message = NULL,
			judged_at = $3, verdict_version = verdict_version + 1,
			version = version + 1, updated_at = $3
		WHERE id = $1 AND latest_generation = $2
			AND latest_generation_phase IN ('claimed','compiling','running','finalizing')
			AND latest_generation_outcome IS NULL`, value.submissionID, value.generation, now)
	if err != nil {
		return fmt.Errorf("close dead generation: %w", err)
	}
	if affected, affectErr := submissionResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	previous := entity.SubmissionState{
		ID: value.submissionID, LatestGeneration: value.generation,
		LatestPhase: entity.Phase(facts.phase), AggregateVersion: facts.version,
	}
	if facts.outcome.Valid {
		previous.LatestOutcome = facts.outcome.String
	}
	if facts.activeJudgementID.Valid {
		previous.ActiveJudgementID = uuidPointer(facts.activeJudgementID.UUID)
	}
	if facts.activeGeneration.Valid {
		previous.ActiveGeneration = int64Pointer(facts.activeGeneration.Int64)
	}
	if facts.activeVerdict.Valid {
		verdict := entity.Verdict(facts.activeVerdict.String)
		previous.ActiveVerdict = &verdict
	}
	next := previous
	next.LatestPhase = entity.PhaseTerminal
	next.LatestOutcome = string(entity.VerdictSystemError)
	next.AggregateVersion++
	payload, marshalErr := json.Marshal(submissioncontract.SubmissionJudgedV1{
		EventID: eventID, SubmissionID: value.submissionID, UserID: facts.userID,
		ProblemID: facts.problemID, Generation: value.generation,
		JudgementID: judgementID, ProblemRevisionID: facts.problemRevisionID,
		TestsetRevisionID: facts.testsetRevisionID, AggregateVersion: next.AggregateVersion,
		LatestPhase: string(next.LatestPhase), LatestOutcome: next.LatestOutcome,
		Verdict: string(entity.VerdictSystemError), PassedCount: 0, TotalCount: 0,
		BecameActive: false, PreviousActive: activeJudgementDTO(previous),
		ResultingActive: activeJudgementDTO(next), SubmittedAt: facts.submittedAt.UTC(),
		OccurredAt: now.UTC(), CorrelationID: facts.originCID,
	})
	if marshalErr != nil {
		return fmt.Errorf("marshal exhausted-generation event: %w", marshalErr)
	}
	entry := audit.Entry{
		Action: "submission.system_error", Resource: submissioncontract.AggregateSubmission,
		ResourceID: value.submissionID.String(), Reason: "attempts_exhausted",
		Before:        map[string]any{"generation": previous.LatestGeneration, "phase": previous.LatestPhase, "outcome": previous.LatestOutcome},
		After:         map[string]any{"generation": next.LatestGeneration, "phase": next.LatestPhase, "outcome": next.LatestOutcome, "judgement_id": judgementID.String(), "failure_classification": "infrastructure", "failure_code": "attempts_exhausted"},
		CorrelationID: facts.originCID, OccurredAt: now.UTC(),
	}
	if err := newDurableFinalizationJournal().Append(ctx, tx, []eventing.Event{{
		ID: eventID, Type: submissioncontract.EventSubmissionJudgedV1,
		AggregateType: submissioncontract.AggregateSubmission,
		AggregateID:   value.submissionID, AggregateVersion: next.AggregateVersion,
		CorrelationID: facts.originCID, Payload: payload, OccurredAt: now.UTC(),
	}}, entry); err != nil {
		return fmt.Errorf("journal exhausted generation: %w", err)
	}
	return nil
}

func (repository *JobRepository) fullJitterBackoff(
	attempt uint32,
	initial time.Duration,
	maximum time.Duration,
) (time.Duration, error) {
	ceiling := initial
	for index := uint32(1); index < attempt && ceiling < maximum; index++ {
		if ceiling > maximum/2 {
			ceiling = maximum
		} else {
			ceiling *= 2
		}
	}
	if ceiling > maximum {
		ceiling = maximum
	}
	value, err := rand.Int(repository.random, big.NewInt(int64(ceiling)+1))
	if err != nil {
		return 0, fmt.Errorf("create retry jitter: %w", err)
	}
	return time.Duration(value.Int64()), nil
}

func (repository *JobRepository) TransitionPhase(
	ctx context.Context,
	lease ports.JobLease,
	expectedSubmissionVersion int64,
	next entity.Phase,
) (entity.SubmissionState, error) {
	if repository == nil || repository.db == nil || ctx == nil ||
		expectedSubmissionVersion < 1 || !phaseInFlight(next) {
		return entity.SubmissionState{}, ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return entity.SubmissionState{}, fmt.Errorf("begin judge phase transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return entity.SubmissionState{}, err
	}
	locked, err := repository.lockFencedLease(ctx, tx, lease, now)
	if err != nil {
		return entity.SubmissionState{}, err
	}
	if locked.submissionVersion != expectedSubmissionVersion ||
		!validPersistedPhaseTransition(entity.Phase(locked.phase), next) {
		return entity.SubmissionState{}, entity.ErrInvalidStateTransition
	}
	attemptState, allowedAttemptStates := persistedAttemptTransition(next)
	attemptResult, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE judge_attempts
		SET state = $2, heartbeat_at = $3, updated_at = $3
		WHERE id = $1 AND state IN (%s)`, allowedAttemptStates),
		lease.Attempt.ID, attemptState, now,
	)
	if err != nil {
		return entity.SubmissionState{}, fmt.Errorf("advance judge attempt phase: %w", err)
	}
	if affected, affectErr := attemptResult.RowsAffected(); affectErr != nil || affected != 1 {
		return entity.SubmissionState{}, ports.ErrLeaseLost
	}
	var (
		state            entity.SubmissionState
		latestOutcome    sql.NullString
		activeGeneration sql.NullInt64
		activeJudgement  uuid.NullUUID
		activeVerdict    sql.NullString
	)
	err = tx.QueryRowContext(ctx, `
		UPDATE submissions
		SET latest_generation_phase = $4, version = version + 1, updated_at = $5
		WHERE id = $1 AND latest_generation = $2 AND version = $3
			AND latest_generation_phase = $6
			AND latest_generation_outcome IS NULL
		RETURNING id, latest_generation, latest_generation_phase,
			latest_generation_outcome, active_judgement_generation,
			active_judgement_id, active_verdict, version`,
		locked.submissionID, locked.generation, expectedSubmissionVersion,
		string(next), now, locked.phase,
	).Scan(
		&state.ID, &state.LatestGeneration, &state.LatestPhase,
		&latestOutcome, &activeGeneration, &activeJudgement,
		&activeVerdict, &state.AggregateVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SubmissionState{}, entity.ErrStaleGeneration
	}
	if err != nil {
		return entity.SubmissionState{}, fmt.Errorf("advance submission generation phase: %w", err)
	}
	state.LatestOutcome = latestOutcome.String
	if activeGeneration.Valid {
		value := activeGeneration.Int64
		state.ActiveGeneration = &value
	}
	if activeJudgement.Valid {
		value := activeJudgement.UUID
		state.ActiveJudgementID = &value
	}
	if activeVerdict.Valid {
		value := entity.Verdict(activeVerdict.String)
		state.ActiveVerdict = &value
	}
	if err := state.Validate(); err != nil {
		return entity.SubmissionState{}, err
	}
	if err := tx.Commit(); err != nil {
		return entity.SubmissionState{}, fmt.Errorf("commit judge phase transition: %w", err)
	}
	return state, nil
}

func (repository *JobRepository) RecordRetry(
	ctx context.Context,
	lease ports.JobLease,
	failure ports.InfrastructureFailure,
) error {
	if repository == nil || repository.db == nil || ctx == nil ||
		!validJobCode(failure.Classification) ||
		!validJobCode(failure.Code) ||
		!validBoundedJobText(failure.Summary, 16<<10) ||
		failure.RetryAfter <= 0 ||
		failure.RetryAfter > maximumLeaseDuration {
		return ErrInvalidJobRepository
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin infrastructure retry: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now, err := jobDatabaseNow(ctx, tx)
	if err != nil {
		return err
	}
	retryAt := now.Add(failure.RetryAfter)
	// RecordRetry can move one leased job back into retry_wait. Acquire the
	// same transaction lock as intake before any durable lease row is locked.
	if err := takeBacklogAdmissionAdvisoryLock(ctx, tx); err != nil {
		return err
	}
	locked, err := repository.lockFencedLease(ctx, tx, lease, now)
	if err != nil {
		return err
	}
	attemptResult, err := tx.ExecContext(ctx, `
		UPDATE judge_attempts
		SET state = 'infrastructure_failed',
			failure_classification = $2, failure_code = $3, failure_summary = $4,
			finished_at = $5, heartbeat_at = $5, updated_at = $5
		WHERE id = $1
			AND state IN ('created','preparing','compiling','running','finalizing')`,
		lease.Attempt.ID, failure.Classification, failure.Code, failure.Summary, now,
	)
	if err != nil {
		return fmt.Errorf("close infrastructure attempt: %w", err)
	}
	if affected, affectErr := attemptResult.RowsAffected(); affectErr != nil || affected != 1 {
		return ports.ErrLeaseLost
	}
	expired := expiredJob{
		jobID: lease.Job.ID, submissionID: locked.submissionID,
		generation: locked.generation, phase: locked.phase,
		attempts: locked.attempts, maxAttempts: locked.maxAttempts,
		attemptID: lease.Attempt.ID, workerID: lease.Attempt.WorkerID,
		workerBootID: lease.Attempt.WorkerBootID,
	}
	if locked.attempts >= locked.maxAttempts {
		if err := repository.deadExpiredJob(ctx, tx, expired, now); err != nil {
			return err
		}
	} else {
		jobResult, updateErr := tx.ExecContext(ctx, `
			UPDATE judge_jobs
			SET state = 'retry_wait', available_at = $2,
				lease_owner = NULL, lease_token_hash = NULL,
				lease_until = NULL, heartbeat_at = NULL,
				last_error_code = $3, last_error_summary = $4, updated_at = $5
			WHERE id = $1 AND state = 'leased'`,
			lease.Job.ID, retryAt, failure.Code, failure.Summary, now,
		)
		if updateErr != nil {
			return fmt.Errorf("schedule infrastructure retry: %w", updateErr)
		}
		if affected, affectErr := jobResult.RowsAffected(); affectErr != nil || affected != 1 {
			return ports.ErrLeaseLost
		}
		submissionResult, updateErr := tx.ExecContext(ctx, `
			UPDATE submissions
			SET latest_generation_phase = 'queued',
				version = version + 1, updated_at = $3
			WHERE id = $1 AND latest_generation = $2
				AND latest_generation_phase IN ('claimed','compiling','running','finalizing')
				AND latest_generation_outcome IS NULL`,
			locked.submissionID, locked.generation, now,
		)
		if updateErr != nil {
			return fmt.Errorf("return infrastructure generation to queue: %w", updateErr)
		}
		if affected, affectErr := submissionResult.RowsAffected(); affectErr != nil || affected != 1 {
			return ports.ErrLeaseLost
		}
	}
	if err := releaseWorkerCapacity(ctx, tx, expired, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infrastructure retry: %w", err)
	}
	return nil
}

type fencedLeaseState struct {
	submissionID      uuid.UUID
	generation        int64
	judgeSpecID       uuid.UUID
	attempts          uint32
	maxAttempts       uint32
	phase             string
	submissionVersion int64
}

func (repository *JobRepository) lockFencedLease(
	ctx context.Context,
	tx *sql.Tx,
	lease ports.JobLease,
	now time.Time,
) (fencedLeaseState, error) {
	if tx == nil || !validJobLeaseCapability(lease) {
		return fencedLeaseState{}, ErrInvalidJobRepository
	}
	var workerHeartbeat time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT worker.heartbeat_at
		FROM judge_worker_instances AS worker
		WHERE worker.id = $1 AND worker.boot_id = $2
			AND NOT EXISTS (
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
			)`,
		lease.Attempt.WorkerID, lease.Attempt.WorkerBootID,
	).Scan(&workerHeartbeat)
	if errors.Is(err, sql.ErrNoRows) {
		return fencedLeaseState{}, ports.ErrWorkerBootLost
	}
	if err != nil {
		return fencedLeaseState{}, fmt.Errorf("validate fenced worker boot: %w", err)
	}
	if workerHeartbeat.After(now) ||
		now.Sub(workerHeartbeat) > repository.freshness {
		return fencedLeaseState{}, ports.ErrWorkerBootLost
	}
	var (
		locked       fencedLeaseState
		jobState     string
		owner        string
		storedHash   []byte
		leaseUntil   time.Time
		workerID     uuid.UUID
		workerBootID uuid.UUID
		attemptState string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT job.submission_id, job.generation, job.judge_spec_id,
			job.attempts, job.max_attempts, job.state, job.lease_owner,
			job.lease_token_hash, job.lease_until,
			attempt.worker_id, attempt.worker_boot_id, attempt.state,
			submission.latest_generation_phase, submission.version
		FROM judge_jobs AS job
		JOIN judge_attempts AS attempt
			ON attempt.id = $2
			AND attempt.job_id = job.id
			AND attempt.submission_id = job.submission_id
			AND attempt.generation = job.generation
			AND attempt.judge_spec_id = job.judge_spec_id
		JOIN submissions AS submission
			ON submission.id = job.submission_id
			AND submission.latest_generation = job.generation
		WHERE job.id = $1
		FOR UPDATE OF job, attempt, submission`,
		lease.Job.ID, lease.Attempt.ID,
	).Scan(
		&locked.submissionID, &locked.generation, &locked.judgeSpecID,
		&locked.attempts, &locked.maxAttempts, &jobState, &owner,
		&storedHash, &leaseUntil, &workerID, &workerBootID, &attemptState,
		&locked.phase, &locked.submissionVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fencedLeaseState{}, ports.ErrLeaseLost
	}
	if err != nil {
		return fencedLeaseState{}, fmt.Errorf("lock fenced judge lease: %w", err)
	}
	presentedHash := sha256.Sum256(lease.LeaseToken)
	normalizedStoredHash := make([]byte, sha256.Size)
	copy(normalizedStoredHash, storedHash)
	hashMatches := subtle.ConstantTimeEq(int32(len(storedHash)), sha256.Size) &
		subtle.ConstantTimeCompare(normalizedStoredHash, presentedHash[:])
	localHashMatches := subtle.ConstantTimeCompare(
		lease.Attempt.LeaseTokenHash[:], presentedHash[:],
	)
	if jobState != string(entity.JobLeased) || hashMatches != 1 ||
		localHashMatches != 1 || !constantTimeTextEqual(owner, lease.LeaseOwner) ||
		!leaseUntil.After(now) || locked.submissionID != lease.Job.SubmissionID ||
		locked.generation != lease.Job.Generation ||
		locked.judgeSpecID != lease.Job.JudgeSpecID ||
		workerID != lease.Attempt.WorkerID ||
		workerBootID != lease.Attempt.WorkerBootID ||
		!persistedAttemptInFlight(attemptState) {
		return fencedLeaseState{}, ports.ErrLeaseLost
	}
	if locked.phase == string(entity.PhaseCancelled) ||
		locked.phase == string(entity.PhaseTerminal) {
		return fencedLeaseState{}, ports.ErrLeaseCancelled
	}
	if locked.phase != string(entity.PhaseClaimed) &&
		locked.phase != string(entity.PhaseCompiling) &&
		locked.phase != string(entity.PhaseRunning) &&
		locked.phase != string(entity.PhaseFinalizing) {
		return fencedLeaseState{}, ports.ErrLeaseLost
	}
	return locked, nil
}

func validJobLeaseCapability(lease ports.JobLease) bool {
	return lease.Job.ID != uuid.Nil && lease.Job.SubmissionID != uuid.Nil &&
		lease.Job.JudgeSpecID != uuid.Nil && lease.Attempt.ID != uuid.Nil &&
		lease.Attempt.WorkerID != uuid.Nil && lease.Attempt.WorkerBootID != uuid.Nil &&
		lease.LeaseOwner != "" && len(lease.LeaseOwner) <= 128 &&
		len(lease.LeaseToken) == leaseTokenBytes && !lease.LeaseUntil.IsZero() &&
		lease.JudgeSpec.ID == lease.Job.JudgeSpecID &&
		lease.JudgeSpec.SubmissionID == lease.Job.SubmissionID &&
		lease.JudgeSpec.Generation == lease.Job.Generation &&
		lease.Attempt.JobID == lease.Job.ID &&
		lease.Attempt.SubmissionID == lease.Job.SubmissionID &&
		lease.Attempt.Generation == lease.Job.Generation &&
		lease.Attempt.JudgeSpecID == lease.Job.JudgeSpecID
}

func matchingLeaseTokenHash(stored []byte, presented [sha256.Size]byte) bool {
	normalized := make([]byte, sha256.Size)
	copy(normalized, stored)
	return subtle.ConstantTimeEq(int32(len(stored)), sha256.Size) == 1 &&
		subtle.ConstantTimeCompare(normalized, presented[:]) == 1
}

func phaseInFlight(phase entity.Phase) bool {
	return phase == entity.PhaseCompiling || phase == entity.PhaseRunning ||
		phase == entity.PhaseFinalizing
}

func validPersistedPhaseTransition(current, next entity.Phase) bool {
	switch current {
	case entity.PhaseClaimed:
		return next == entity.PhaseCompiling
	case entity.PhaseCompiling:
		return next == entity.PhaseRunning || next == entity.PhaseFinalizing
	case entity.PhaseRunning:
		return next == entity.PhaseFinalizing
	default:
		return false
	}
}

func persistedAttemptTransition(phase entity.Phase) (string, string) {
	switch phase {
	case entity.PhaseCompiling:
		return string(entity.AttemptCompiling), "'created','preparing'"
	case entity.PhaseRunning:
		return string(entity.AttemptRunning), "'compiling'"
	case entity.PhaseFinalizing:
		return string(entity.AttemptFinalizing), "'compiling','running'"
	default:
		return "", "''"
	}
}

func persistedAttemptInFlight(state string) bool {
	return state == string(entity.AttemptCreated) ||
		state == string(entity.AttemptPreparing) ||
		state == string(entity.AttemptCompiling) ||
		state == string(entity.AttemptRunning) ||
		state == string(entity.AttemptFinalizing)
}

func validBoundedJobText(value string, maximum int) bool {
	if len(value) == 0 || len(value) > maximum || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
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

func validJobCode(value string) bool {
	if len(value) == 0 || len(value) > 64 ||
		value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func constantTimeTextEqual(left, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func jobDatabaseNow(ctx context.Context, tx *sql.Tx) (time.Time, error) {
	var now time.Time
	if err := tx.QueryRowContext(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return time.Time{}, fmt.Errorf("read database clock: %w", err)
	}
	return now.UTC(), nil
}

func takeClaimAdvisoryLock(ctx context.Context, tx *sql.Tx) error {
	var ignored any
	if err := tx.QueryRowContext(ctx, "SELECT pg_advisory_xact_lock($1)", claimAdvisoryLock).Scan(&ignored); err != nil {
		return fmt.Errorf("lock judge admission bounds: %w", err)
	}
	return nil
}

func takeBacklogAdmissionAdvisoryLock(ctx context.Context, tx *sql.Tx) error {
	var ignored any
	if err := tx.QueryRowContext(
		ctx,
		"SELECT pg_advisory_xact_lock($1)",
		submissioncontract.JudgeBacklogAdmissionAdvisoryLock,
	).Scan(&ignored); err != nil {
		return fmt.Errorf("lock judge backlog admission: %w", err)
	}
	return nil
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func wipeRepositoryLeaseTokens(leases []ports.JobLease) {
	for index := range leases {
		wipeBytes(leases[index].LeaseToken)
		leases[index].LeaseToken = nil
	}
}

func wipeBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

var _ ports.JudgeJobRepository = (*JobRepository)(nil)
