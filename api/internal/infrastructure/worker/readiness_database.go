package worker

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	"github.com/huynhanx03/judgify/internal/judge/capability"
)

const persistentReadinessSnapshotReuse = 100 * time.Millisecond

const workerLeaseReadinessSQL = `
SELECT count(*) FILTER (
           WHERE job.state = 'leased'
             AND attempt.worker_id = $1
             AND attempt.worker_boot_id = $2
       )::bigint,
       COALESCE(bool_and(COALESCE(
           job.state = 'leased'
           AND job.lease_owner = $5
           AND octet_length(job.lease_token_hash) = 32
           AND attempt.id IS NOT NULL
           AND attempt.worker_id = $1
           AND attempt.worker_boot_id = $2
           AND attempt.lease_owner = $5
           AND octet_length(attempt.lease_token_hash) = 32
           AND job.lease_token_hash = attempt.lease_token_hash
           AND job.lease_until IS NOT NULL
           AND job.lease_until > $3
           AND job.heartbeat_at IS NOT NULL
           AND job.heartbeat_at >= $4
           AND job.heartbeat_at <= $3
           AND attempt.heartbeat_at IS NOT NULL
           AND attempt.heartbeat_at >= $4
           AND attempt.heartbeat_at <= $3
           AND attempt.state IN (
               'created', 'preparing', 'compiling', 'running', 'finalizing'
           )
       , false)), true)
FROM judge_jobs AS job
LEFT JOIN judge_attempts AS attempt
  ON attempt.job_id = job.id
 AND attempt.attempt_number = job.attempts
WHERE (
        job.state = 'leased'
        AND job.lease_owner = $5
      )
   OR (
        attempt.worker_id = $1
        AND attempt.worker_boot_id = $2
        AND attempt.state IN (
            'created', 'preparing', 'compiling', 'running', 'finalizing'
        )
      )`

// ReadinessSQL is the read-only SQL boundary used by worker readiness.
// *sql.DB satisfies it. Callers retain ownership of the pool.
type ReadinessSQL interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type readinessQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// PersistentReadinessSnapshot exports only bounded, non-identifying derived
// state. Worker/boot IDs, tuple hashes, paths, raw timestamps, and database
// errors remain private and must never become metric or health payload fields.
type PersistentReadinessSnapshot struct {
	DatabaseClockHealthy     bool
	CurrentBoot              bool
	HeartbeatFresh           bool
	ClaimsEnabled            bool
	CapacityBounded          bool
	LeasesFresh              bool
	TotalCapacity            int
	FreeCapacity             int
	ActiveLeaseCount         int
	ExpectedCapabilityCount  int
	PersistedCapabilityCount int
	ExactReleasePersisted    bool
	EvidenceBootBound        bool

	databaseTime  time.Time
	bootStartedAt time.Time
	heartbeatAt   time.Time
}

type PersistentReadinessSnapshotter interface {
	Snapshot(context.Context) (PersistentReadinessSnapshot, error)
}

// SQLPersistentReadinessSnapshotter proves the same exact boot and tuple
// fields used by durable claim fencing without mutating leases or capability
// evidence. Concurrent health facets coalesce around one bounded SQL snapshot.
type SQLPersistentReadinessSnapshotter struct {
	database ReadinessSQL
	state    *RuntimeReadinessState
	now      func() time.Time

	mu       sync.Mutex
	loading  chan struct{}
	cachedAt time.Time
	cached   PersistentReadinessSnapshot
	cacheErr error
}

func NewSQLPersistentReadinessSnapshotter(
	database ReadinessSQL,
	state *RuntimeReadinessState,
) (*SQLPersistentReadinessSnapshotter, error) {
	if nilInterface(database) || state == nil {
		return nil, ErrWorkerReadiness
	}
	return &SQLPersistentReadinessSnapshotter{
		database: database,
		state:    state,
		now:      time.Now,
	}, nil
}

func (snapshotter *SQLPersistentReadinessSnapshotter) Snapshot(
	ctx context.Context,
) (PersistentReadinessSnapshot, error) {
	if snapshotter == nil || nilInterface(snapshotter.database) ||
		snapshotter.state == nil || snapshotter.now == nil || ctx == nil {
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	if err := ctx.Err(); err != nil {
		return PersistentReadinessSnapshot{}, err
	}
	for {
		now := snapshotter.now().UTC()
		snapshotter.mu.Lock()
		if snapshotter.cachedAtIsReusable(now) {
			snapshot, err := snapshotter.cached, snapshotter.cacheErr
			snapshotter.mu.Unlock()
			return snapshot, err
		}
		if snapshotter.loading != nil {
			wait := snapshotter.loading
			snapshotter.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return PersistentReadinessSnapshot{}, ctx.Err()
			}
		}
		loading := make(chan struct{})
		snapshotter.loading = loading
		snapshotter.mu.Unlock()

		snapshot, err := snapshotter.load(ctx)
		completedAt := snapshotter.now().UTC()
		cacheable := !errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded)

		snapshotter.mu.Lock()
		if cacheable {
			snapshotter.cachedAt = completedAt
			snapshotter.cached = snapshot
			snapshotter.cacheErr = err
		} else {
			snapshotter.cachedAt = time.Time{}
			snapshotter.cached = PersistentReadinessSnapshot{}
			snapshotter.cacheErr = nil
		}
		snapshotter.loading = nil
		close(loading)
		snapshotter.mu.Unlock()
		return snapshot, err
	}
}

func (snapshotter *SQLPersistentReadinessSnapshotter) cachedAtIsReusable(
	now time.Time,
) bool {
	return !snapshotter.cachedAt.IsZero() &&
		!now.Before(snapshotter.cachedAt) &&
		now.Sub(snapshotter.cachedAt) <= persistentReadinessSnapshotReuse
}

func (snapshotter *SQLPersistentReadinessSnapshotter) load(
	ctx context.Context,
) (PersistentReadinessSnapshot, error) {
	binding, bound := snapshotter.state.persistenceBinding()
	if !bound {
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	tx, err := snapshotter.database.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return PersistentReadinessSnapshot{}, ctx.Err()
		}
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	defer func() { _ = tx.Rollback() }()
	snapshot, workerFound, err := snapshotter.loadWorker(ctx, tx, binding)
	if err != nil || !workerFound {
		return snapshot, err
	}
	snapshot, err = snapshotter.loadCapabilities(ctx, tx, binding, snapshot)
	if err != nil {
		return PersistentReadinessSnapshot{}, err
	}
	snapshot, err = snapshotter.loadLeases(ctx, tx, binding, snapshot)
	if err != nil {
		return PersistentReadinessSnapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		if ctx.Err() != nil {
			return PersistentReadinessSnapshot{}, ctx.Err()
		}
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	return snapshot, nil
}

func (snapshotter *SQLPersistentReadinessSnapshotter) loadWorker(
	ctx context.Context,
	database readinessQueryer,
	binding readinessBinding,
) (PersistentReadinessSnapshot, bool, error) {
	rows, err := database.QueryContext(ctx, `
SELECT clock_timestamp(),
       worker.boot_started_at,
       worker.heartbeat_at,
       worker.claims_enabled,
       worker.total_capacity,
       worker.free_capacity,
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
       ) AS current_boot
FROM judge_worker_instances AS worker
WHERE worker.id = $1
  AND worker.boot_id = $2
  AND worker.worker_key = $3
LIMIT 1`,
		binding.worker.ID,
		binding.worker.BootID,
		binding.worker.WorkerKey,
	)
	if err != nil {
		if ctx.Err() != nil {
			return PersistentReadinessSnapshot{}, false, ctx.Err()
		}
		return PersistentReadinessSnapshot{}, false, ErrWorkerReadiness
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return PersistentReadinessSnapshot{}, false, ErrWorkerReadiness
		}
		return PersistentReadinessSnapshot{}, false, nil
	}

	var (
		snapshot      PersistentReadinessSnapshot
		totalCapacity int
		freeCapacity  int
	)
	if err := rows.Scan(
		&snapshot.databaseTime,
		&snapshot.bootStartedAt,
		&snapshot.heartbeatAt,
		&snapshot.ClaimsEnabled,
		&totalCapacity,
		&freeCapacity,
		&snapshot.CurrentBoot,
	); err != nil {
		return PersistentReadinessSnapshot{}, false, ErrWorkerReadiness
	}
	if rows.Next() || rows.Err() != nil {
		return PersistentReadinessSnapshot{}, false, ErrWorkerReadiness
	}
	snapshot.databaseTime = snapshot.databaseTime.UTC()
	snapshot.bootStartedAt = snapshot.bootStartedAt.UTC()
	snapshot.heartbeatAt = snapshot.heartbeatAt.UTC()
	snapshot.ExpectedCapabilityCount = len(binding.expected)
	snapshot.DatabaseClockHealthy =
		!snapshot.databaseTime.IsZero() &&
			!snapshot.bootStartedAt.IsZero() &&
			!snapshot.heartbeatAt.IsZero() &&
			!snapshot.bootStartedAt.After(snapshot.heartbeatAt) &&
			!snapshot.heartbeatAt.After(snapshot.databaseTime)
	snapshot.HeartbeatFresh =
		snapshot.DatabaseClockHealthy &&
			snapshot.heartbeatAt.After(snapshot.bootStartedAt) &&
			snapshot.databaseTime.Sub(snapshot.heartbeatAt) <= binding.heartbeatFreshness
	snapshot.CapacityBounded =
		totalCapacity == binding.worker.TotalCapacity &&
			totalCapacity > 0 &&
			freeCapacity >= 0 &&
			freeCapacity <= totalCapacity
	snapshot.TotalCapacity = totalCapacity
	snapshot.FreeCapacity = freeCapacity
	return snapshot, true, nil
}

func (snapshotter *SQLPersistentReadinessSnapshotter) loadCapabilities(
	ctx context.Context,
	database readinessQueryer,
	binding readinessBinding,
	snapshot PersistentReadinessSnapshot,
) (PersistentReadinessSnapshot, error) {
	rows, err := database.QueryContext(ctx, `
SELECT evidence.judge_spec_protocol,
       evidence.runtime_key,
       evidence.runtime_profile_version,
       evidence.compiler_version,
       evidence.runtime_bootstrap_version,
       evidence.compile_command_id,
       evidence.run_command_id,
       evidence.image_digest,
       evidence.manifest_version,
       evidence.manifest_checksum,
       evidence.supervisor_protocol,
       evidence.checker_key,
       evidence.checker_version,
       evidence.checker_implementation_checksum,
       evidence.capability_profile_hash,
       evidence.smoke_checksum,
       evidence.golden_checksum,
       evidence.probed_at,
       evidence.passing
FROM judge_worker_capabilities AS evidence
WHERE evidence.worker_instance_id = $1
  AND evidence.boot_id = $2
ORDER BY evidence.capability_profile_hash
LIMIT $3`,
		binding.worker.ID,
		binding.worker.BootID,
		len(binding.expected)+1,
	)
	if err != nil {
		if ctx.Err() != nil {
			return PersistentReadinessSnapshot{}, ctx.Err()
		}
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	defer func() { _ = rows.Close() }()

	worker := binding.worker
	worker.StartedAt = snapshot.bootStartedAt
	worker.HeartbeatAt = snapshot.heartbeatAt
	worker.ClaimsEnabled = snapshot.ClaimsEnabled
	seen := make(map[string]struct{}, len(binding.expected))
	exact := true
	bootBound := !snapshot.databaseTime.IsZero() &&
		!snapshot.bootStartedAt.IsZero() &&
		!snapshot.bootStartedAt.After(snapshot.databaseTime)
	for rows.Next() {
		var evidence capability.CapabilityEvidence
		evidence.BootID = binding.worker.BootID
		if err := rows.Scan(
			&evidence.Tuple.JudgeSpecProtocol,
			&evidence.Tuple.RuntimeKey,
			&evidence.Tuple.RuntimeProfileVersion,
			&evidence.Tuple.CompilerVersion,
			&evidence.Tuple.RuntimeBootstrapVersion,
			&evidence.Tuple.CompileCommandID,
			&evidence.Tuple.RunCommandID,
			&evidence.Tuple.ImageDigest,
			&evidence.Tuple.ManifestVersion,
			&evidence.Tuple.ManifestChecksum,
			&evidence.Tuple.SupervisorProtocol,
			&evidence.Tuple.CheckerKey,
			&evidence.Tuple.CheckerVersion,
			&evidence.Tuple.CheckerImplementationChecksum,
			&evidence.Tuple.CapabilityProfileHash,
			&evidence.SmokeChecksum,
			&evidence.GoldenChecksum,
			&evidence.ProbedAt,
			&evidence.Passing,
		); err != nil {
			return PersistentReadinessSnapshot{}, ErrWorkerReadiness
		}
		snapshot.PersistedCapabilityCount++
		evidence.ProbedAt = evidence.ProbedAt.UTC()
		evidence.CheckerImplementationChecksum =
			evidence.Tuple.CheckerImplementationChecksum
		expected, exists := binding.expected[evidence.Tuple.CapabilityProfileHash]
		_, duplicate := seen[evidence.Tuple.CapabilityProfileHash]
		if !exists || duplicate || expected != evidence.Tuple ||
			capability.ValidatePassingEvidence(worker, evidence) != nil ||
			evidence.ProbedAt.After(snapshot.databaseTime) {
			exact = false
		}
		seen[evidence.Tuple.CapabilityProfileHash] = struct{}{}
		if evidence.ProbedAt.IsZero() ||
			evidence.ProbedAt.Before(snapshot.bootStartedAt) ||
			evidence.ProbedAt.After(snapshot.databaseTime) ||
			!evidence.Passing {
			bootBound = false
		}
	}
	if err := rows.Err(); err != nil {
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	if snapshot.PersistedCapabilityCount != len(binding.expected) ||
		len(seen) != len(binding.expected) {
		exact = false
		bootBound = false
	}
	snapshot.EvidenceBootBound = bootBound
	snapshot.ExactReleasePersisted = exact && bootBound
	return snapshot, nil
}

func (snapshotter *SQLPersistentReadinessSnapshotter) loadLeases(
	ctx context.Context,
	database readinessQueryer,
	binding readinessBinding,
	snapshot PersistentReadinessSnapshot,
) (PersistentReadinessSnapshot, error) {
	rows, err := database.QueryContext(ctx, workerLeaseReadinessSQL,
		binding.worker.ID,
		binding.worker.BootID,
		snapshot.databaseTime,
		snapshot.databaseTime.Add(-binding.heartbeatFreshness),
		binding.worker.WorkerKey,
	)
	if err != nil {
		if ctx.Err() != nil {
			return PersistentReadinessSnapshot{}, ctx.Err()
		}
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return PersistentReadinessSnapshot{}, ErrWorkerReadiness
		}
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	var (
		active int64
		fresh  bool
	)
	if err := rows.Scan(&active, &fresh); err != nil ||
		active < 0 || active > int64(snapshot.TotalCapacity) {
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	if rows.Next() || rows.Err() != nil {
		return PersistentReadinessSnapshot{}, ErrWorkerReadiness
	}
	snapshot.ActiveLeaseCount = int(active)
	snapshot.LeasesFresh = fresh
	if snapshot.FreeCapacity != snapshot.TotalCapacity-snapshot.ActiveLeaseCount {
		snapshot.CapacityBounded = false
	}
	return snapshot, nil
}

type persistentReadinessFacet uint8

const (
	persistentCurrentBootFacet persistentReadinessFacet = iota + 1
	persistentLeaseHeartbeatFacet
	persistentClaimsEnabledFacet
	persistentExactReleaseFacet
)

type persistentReadinessChecker struct {
	snapshotter PersistentReadinessSnapshotter
	facet       persistentReadinessFacet
}

func (checker persistentReadinessChecker) Check(ctx context.Context) error {
	if nilInterface(checker.snapshotter) || ctx == nil {
		return ErrWorkerReadiness
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot, err := checker.snapshotter.Snapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrWorkerReadiness
	}
	switch checker.facet {
	case persistentCurrentBootFacet:
		if snapshot.DatabaseClockHealthy && snapshot.CurrentBoot &&
			snapshot.HeartbeatFresh {
			return nil
		}
	case persistentLeaseHeartbeatFacet:
		if snapshot.LeasesFresh && snapshot.CapacityBounded {
			return nil
		}
	case persistentClaimsEnabledFacet:
		if snapshot.ClaimsEnabled {
			return nil
		}
	case persistentExactReleaseFacet:
		if snapshot.ExactReleasePersisted && snapshot.EvidenceBootBound {
			return nil
		}
	}
	return ErrWorkerReadiness
}

// PersistentReadinessCheckers are stable adapters for the exact current boot.
type PersistentReadinessCheckers struct {
	CurrentBoot    commonhealth.Checker
	LeaseHeartbeat commonhealth.Checker
	ClaimsEnabled  commonhealth.Checker
	ExactRelease   commonhealth.Checker
}

func NewPersistentReadinessCheckers(
	snapshotter PersistentReadinessSnapshotter,
) (PersistentReadinessCheckers, error) {
	if nilInterface(snapshotter) {
		return PersistentReadinessCheckers{}, ErrWorkerReadiness
	}
	return PersistentReadinessCheckers{
		CurrentBoot: persistentReadinessChecker{
			snapshotter: snapshotter,
			facet:       persistentCurrentBootFacet,
		},
		LeaseHeartbeat: persistentReadinessChecker{
			snapshotter: snapshotter,
			facet:       persistentLeaseHeartbeatFacet,
		},
		ClaimsEnabled: persistentReadinessChecker{
			snapshotter: snapshotter,
			facet:       persistentClaimsEnabledFacet,
		},
		ExactRelease: persistentReadinessChecker{
			snapshotter: snapshotter,
			facet:       persistentExactReleaseFacet,
		},
	}, nil
}

var (
	_ PersistentReadinessSnapshotter = (*SQLPersistentReadinessSnapshotter)(nil)
	_ commonhealth.Checker           = persistentReadinessChecker{}
)
