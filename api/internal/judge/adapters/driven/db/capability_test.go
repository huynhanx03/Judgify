package db

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/testutil/dbtest"
)

func TestCapabilityEvidenceStoreAtomicallyPersistsExactEvidenceSetAndEnablesSameBoot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}

	startedAt := time.Date(2026, time.July, 28, 9, 0, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-a", uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))
	second := capabilityTestEvidenceVariant(
		t,
		worker.BootID,
		startedAt.Add(2*time.Second),
	)
	if err := store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence, second},
	); err != nil {
		t.Fatalf("RecordPassingEvidenceSetAndEnableClaims() error = %v", err)
	}

	var enabled bool
	if err := database.QueryRowContext(ctx, `SELECT claims_enabled FROM judge_worker_instances WHERE id = ? AND boot_id = ?`, worker.ID, worker.BootID).Scan(&enabled); err != nil {
		t.Fatalf("read worker claims state: %v", err)
	}
	if !enabled {
		t.Fatal("same worker boot claims remain disabled after committed evidence")
	}
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM judge_worker_capabilities WHERE worker_instance_id = ? AND boot_id = ? AND capability_profile_hash = ?`, worker.ID, worker.BootID, evidence.Tuple.CapabilityProfileHash).Scan(&count); err != nil {
		t.Fatalf("count exact evidence: %v", err)
	}
	if count != 1 {
		t.Fatalf("first exact evidence row count = %d, want 1", count)
	}
	assertCapabilityPersistence(t, database, worker, true, 2)
}

func TestCapabilityEvidenceStoreRejectsWrongBootWithoutWritingEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 9, 5, 0, 0, time.UTC)
	registered := capability.NewWorkerInstance(uuid.New(), "worker-wrong-boot", uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, registered); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	stale := capability.NewWorkerInstance(registered.ID, registered.WorkerKey, uuid.New(), startedAt)
	if err := store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		stale,
		[]capability.CapabilityEvidence{
			capabilityTestEvidence(t, stale.BootID, startedAt.Add(time.Second)),
		},
	); err == nil {
		t.Fatal("wrong boot evidence unexpectedly committed")
	}
	assertCapabilityPersistence(t, database, registered, false, 0)
}

func TestCapabilityEvidenceStoreRollsBackClaimEnableWhenEvidenceInsertFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 9, 10, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-rollback", uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))
	existing := capabilityTestEvidenceVariant(
		t,
		worker.BootID,
		startedAt.Add(2*time.Second),
	)
	insertCapabilityEvidenceDirectly(t, client, worker, existing)
	if err := store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence, existing},
	); err == nil {
		t.Fatal("partially conflicting evidence set unexpectedly enabled claims")
	}
	assertCapabilityPersistence(t, database, worker, false, 1)
}

func TestCapabilityEvidenceStoreReconstructionReadsDurableState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 9, 15, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-restart", uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	if err := store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{
			capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second)),
		},
	); err != nil {
		t.Fatalf("RecordPassingEvidenceSetAndEnableClaims() error = %v", err)
	}
	// Reconstruct the production adapter: eligibility remains database state,
	// not an in-memory side effect of the first store value.
	if _, err := NewCapabilityEvidenceStore(client); err != nil {
		t.Fatalf("reconstruct capability store: %v", err)
	}
	assertCapabilityPersistence(t, database, worker, true, 1)
}

func TestCapabilityEvidenceStoreConcurrentAttemptsHaveOneWinner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 9, 20, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-concurrent", uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))

	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var attempts sync.WaitGroup
	for range 2 {
		attempts.Add(1)
		go func() {
			defer attempts.Done()
			<-start
			errorsByAttempt <- store.RecordPassingEvidenceSetAndEnableClaims(
				ctx,
				worker,
				[]capability.CapabilityEvidence{evidence},
			)
		}()
	}
	close(start)
	attempts.Wait()
	close(errorsByAttempt)
	successes := 0
	for err := range errorsByAttempt {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent attempts = %d, want 1", successes)
	}
	assertCapabilityPersistence(t, database, worker, true, 1)
}

func TestCapabilityEvidenceStorePostgresTransactionRollbackAndConcurrentWinner(t *testing.T) {
	ctx, client, database := openCapabilityPostgres(t)
	store, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 9, 25, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-pg-"+uuid.NewString()[:8], uuid.New(), startedAt)
	if err := store.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM judge_worker_capabilities WHERE worker_instance_id = $1`, worker.ID)
		_, _ = database.ExecContext(context.Background(), `DELETE FROM judge_worker_instances WHERE id = $1`, worker.ID)
	})
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))

	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var attempts sync.WaitGroup
	for range 2 {
		attempts.Add(1)
		go func() {
			defer attempts.Done()
			<-start
			errorsByAttempt <- store.RecordPassingEvidenceSetAndEnableClaims(
				ctx,
				worker,
				[]capability.CapabilityEvidence{evidence},
			)
		}()
	}
	close(start)
	attempts.Wait()
	close(errorsByAttempt)
	successes := 0
	for err := range errorsByAttempt {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful PostgreSQL attempts = %d, want 1", successes)
	}
	assertCapabilityPostgresPersistence(t, ctx, database, worker, true, 1)

	if err := store.DisableClaims(ctx, worker.ID, worker.BootID); err != nil {
		t.Fatalf("DisableClaims() error = %v", err)
	}
	if err := store.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence},
	); err == nil {
		t.Fatal("duplicate PostgreSQL evidence unexpectedly enabled claims")
	}
	assertCapabilityPostgresPersistence(t, ctx, database, worker, false, 1)
}

func TestPlatformActivationStoreProvesFreshExactWorkerAndPersistsRetirement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	evidenceStore, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	activationStore, err := NewPlatformActivationStore(client)
	if err != nil {
		t.Fatalf("NewPlatformActivationStore() error = %v", err)
	}
	service, err := capability.NewActivationService(activationStore, 2*time.Minute)
	if err != nil {
		t.Fatalf("NewActivationService() error = %v", err)
	}
	startedAt := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(uuid.New(), "worker-platform-activation", uuid.New(), startedAt)
	if err := evidenceStore.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(10*time.Second))
	if err := evidenceStore.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence},
	); err != nil {
		t.Fatalf("RecordPassingEvidenceSetAndEnableClaims() error = %v", err)
	}
	refreshCapabilityTestWorkerHeartbeat(
		t,
		ctx,
		database,
		worker,
		startedAt.Add(20*time.Second),
	)
	request := capability.ActivationRequest{
		ReleaseID:                       "judge-release.2026.07.28.1",
		ApprovedReleaseManifestChecksum: "sha256:5620a451a0cc99f144364050e8896240f665461f32ac4a6e84ba8572b2361357",
		ActorID:                         uuid.New(), AuditID: uuid.New(),
		Reason: "approve exact launch capability after reviewed golden smoke evidence",
		Tuple:  evidence.Tuple,
	}
	activated, err := service.Activate(ctx, request, startedAt.Add(30*time.Second))
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if activated.ProofWorkerInstanceID != worker.ID || activated.ProofBootID != worker.BootID || activated.ProofEvidenceID == uuid.Nil {
		t.Fatalf("activation proof = (%s, %s, %s), want exact worker boot evidence", activated.ProofWorkerInstanceID, activated.ProofBootID, activated.ProofEvidenceID)
	}
	if _, err := service.Activate(ctx, request, startedAt.Add(31*time.Second)); err == nil {
		t.Fatal("duplicate active release tuple unexpectedly succeeded")
	}

	active, exists, err := activationStore.Active(ctx, request.ReleaseID, request.Tuple)
	if err != nil || !exists || active.ID != activated.ID {
		t.Fatalf("Active() = (%#v, %v, %v), want exact durable activation", active, exists, err)
	}
	if _, err := database.Exec(
		`UPDATE activated_judge_capabilities SET compile_command_id = ? WHERE id = ?`,
		"cpp17.compile.v2",
		activated.ID,
	); err == nil {
		t.Fatal("database accepted activation proof with a different executable identity")
	}
	retiredAt := startedAt.Add(time.Minute)
	retirement := capability.RetirementRequest{
		ActivationID: activated.ID, ActorID: uuid.New(), AuditID: uuid.New(),
		Reason: "retire reviewed release capability before replacement activation",
	}
	if err := service.Retire(ctx, retirement, retiredAt); err != nil {
		t.Fatalf("Retire() error = %v", err)
	}
	if _, exists, err := activationStore.Active(ctx, request.ReleaseID, request.Tuple); err != nil || exists {
		t.Fatalf("Active() after retirement = (_, %v, %v), want absent", exists, err)
	}
	var state string
	var persistedRetiredAt time.Time
	var retiredBy, retirementAuditID uuid.UUID
	var reason string
	if err := database.QueryRowContext(ctx, `SELECT state, retired_at, retired_by, retirement_audit_id, retirement_reason FROM activated_judge_capabilities WHERE id = ?`, activated.ID).
		Scan(&state, &persistedRetiredAt, &retiredBy, &retirementAuditID, &reason); err != nil {
		t.Fatalf("read retired activation: %v", err)
	}
	if state != "retired" || !persistedRetiredAt.Equal(retiredAt) || retiredBy != retirement.ActorID ||
		retirementAuditID != retirement.AuditID || reason != retirement.Reason {
		t.Fatalf("retirement audit = (%q, %v, %s, %s, %q), want exact command", state, persistedRetiredAt, retiredBy, retirementAuditID, reason)
	}

	// Liveness is always queried from the database. A caller cannot reuse the
	// earlier proof after claims are disabled.
	if err := evidenceStore.DisableClaims(ctx, worker.ID, worker.BootID); err != nil {
		t.Fatalf("DisableClaims() error = %v", err)
	}
	request.ReleaseID = "judge-release.2026.07.28.2"
	if _, err := service.Activate(ctx, request, startedAt.Add(90*time.Second)); err == nil {
		t.Fatal("activation without a claims-enabled worker unexpectedly succeeded")
	}
}

func TestPlatformActivationStoreAcceptsBootBoundEvidenceOlderThanHeartbeatWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	evidenceStore, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	activationStore, err := NewPlatformActivationStore(client)
	if err != nil {
		t.Fatalf("NewPlatformActivationStore() error = %v", err)
	}
	service, err := capability.NewActivationService(activationStore, 30*time.Second)
	if err != nil {
		t.Fatalf("NewActivationService() error = %v", err)
	}

	startedAt := time.Date(2026, time.July, 28, 11, 0, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(
		uuid.New(),
		"worker-platform-immutable-evidence",
		uuid.New(),
		startedAt,
	)
	if err := evidenceStore.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))
	if err := evidenceStore.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence},
	); err != nil {
		t.Fatalf("RecordPassingEvidenceSetAndEnableClaims() error = %v", err)
	}

	activatedAt := startedAt.Add(5 * time.Minute)
	heartbeatAt := activatedAt.Add(-10 * time.Second)
	refreshCapabilityTestWorkerHeartbeat(t, ctx, database, worker, heartbeatAt)

	request := capability.ActivationRequest{
		ReleaseID:                       "judge-release.2026.07.28.immutable-evidence",
		ApprovedReleaseManifestChecksum: "sha256:a4c92876b922a1c230778addf228569681562371233573bac9965326e9f911e5",
		ActorID:                         uuid.New(),
		AuditID:                         uuid.New(),
		Reason:                          "activate exact immutable boot evidence while worker heartbeat remains fresh",
		Tuple:                           evidence.Tuple,
	}
	activated, err := service.Activate(ctx, request, activatedAt)
	if err != nil {
		t.Fatalf("Activate() with boot-bound evidence older than heartbeat window error = %v", err)
	}
	if activated.ProofWorkerInstanceID != worker.ID ||
		activated.ProofBootID != worker.BootID ||
		activated.ProofEvidenceID == uuid.Nil {
		t.Fatalf(
			"activation proof = (%s, %s, %s), want exact worker boot evidence",
			activated.ProofWorkerInstanceID,
			activated.ProofBootID,
			activated.ProofEvidenceID,
		)
	}
}

func TestPlatformActivationStoreRequiresCurrentBootPostRegistrationHeartbeat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client, database := openCapabilitySQLite(t)
	evidenceStore, err := NewCapabilityEvidenceStore(client)
	if err != nil {
		t.Fatalf("NewCapabilityEvidenceStore() error = %v", err)
	}
	activationStore, err := NewPlatformActivationStore(client)
	if err != nil {
		t.Fatalf("NewPlatformActivationStore() error = %v", err)
	}
	service, err := capability.NewActivationService(activationStore, 30*time.Second)
	if err != nil {
		t.Fatalf("NewActivationService() error = %v", err)
	}

	startedAt := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	worker := capability.NewWorkerInstance(
		uuid.New(),
		"worker-platform-current-boot",
		uuid.New(),
		startedAt,
	)
	if err := evidenceStore.RegisterBoot(ctx, worker); err != nil {
		t.Fatalf("RegisterBoot() error = %v", err)
	}
	evidence := capabilityTestEvidence(t, worker.BootID, startedAt.Add(time.Second))
	if err := evidenceStore.RecordPassingEvidenceSetAndEnableClaims(
		ctx,
		worker,
		[]capability.CapabilityEvidence{evidence},
	); err != nil {
		t.Fatalf("RecordPassingEvidenceSetAndEnableClaims() error = %v", err)
	}
	request := capability.ActivationRequest{
		ReleaseID:                       "judge-release.2026.07.28.current-boot",
		ApprovedReleaseManifestChecksum: "sha256:93a8a6af2621cb032a332e2340acf238768e4ff7f614b37b7bb0e6666c97331d",
		ActorID:                         uuid.New(),
		AuditID:                         uuid.New(),
		Reason:                          "require a current worker boot with a durable post-registration heartbeat",
		Tuple:                           evidence.Tuple,
	}
	if _, err := service.Activate(ctx, request, startedAt.Add(10*time.Second)); err == nil {
		t.Fatal("registration-only worker heartbeat unexpectedly authorized activation")
	}

	refreshCapabilityTestWorkerHeartbeat(
		t,
		ctx,
		database,
		worker,
		startedAt.Add(9*time.Second),
	)
	newer := capability.NewWorkerInstance(
		uuid.New(),
		worker.WorkerKey,
		uuid.New(),
		startedAt.Add(6*time.Second),
	)
	if err := evidenceStore.RegisterBoot(ctx, newer); err != nil {
		t.Fatalf("RegisterBoot() newer boot error = %v", err)
	}
	if _, err := service.Activate(ctx, request, startedAt.Add(10*time.Second)); err == nil {
		t.Fatal("superseded claims-enabled worker boot unexpectedly authorized activation")
	}
}

func refreshCapabilityTestWorkerHeartbeat(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	worker capability.WorkerInstance,
	heartbeatAt time.Time,
) {
	t.Helper()
	result, err := database.ExecContext(
		ctx,
		`UPDATE judge_worker_instances
		 SET heartbeat_at = ?, updated_at = ?
		 WHERE id = ? AND boot_id = ?`,
		heartbeatAt,
		heartbeatAt,
		worker.ID,
		worker.BootID,
	)
	if err != nil {
		t.Fatalf("refresh worker heartbeat: %v", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("read refreshed worker heartbeat row count: %v", err)
	}
	if updated != 1 {
		t.Fatalf("refreshed worker heartbeat rows = %d, want 1", updated)
	}
}

func openCapabilityPostgres(t *testing.T) (context.Context, *internalent.EntClient, *sql.DB) {
	t.Helper()
	dsn := dbtest.RequireDatabaseURL(t)
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open(postgres) error = %v", err)
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("database.PingContext() error = %v", err)
	}
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	return ctx, internalent.WrapClientWithDriver(client, driver, nil), database
}

func assertCapabilityPostgresPersistence(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	worker capability.WorkerInstance,
	wantEnabled bool,
	wantEvidence int,
) {
	t.Helper()
	var enabled bool
	if err := database.QueryRowContext(ctx, `SELECT claims_enabled FROM judge_worker_instances WHERE id = $1 AND boot_id = $2`, worker.ID, worker.BootID).Scan(&enabled); err != nil {
		t.Fatalf("read PostgreSQL claims state: %v", err)
	}
	if enabled != wantEnabled {
		t.Fatalf("PostgreSQL claims_enabled = %v, want %v", enabled, wantEnabled)
	}
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM judge_worker_capabilities WHERE worker_instance_id = $1 AND boot_id = $2`, worker.ID, worker.BootID).Scan(&count); err != nil {
		t.Fatalf("count PostgreSQL worker evidence: %v", err)
	}
	if count != wantEvidence {
		t.Fatalf("PostgreSQL evidence count = %d, want %d", count, wantEvidence)
	}
}

func insertCapabilityEvidenceDirectly(
	t *testing.T,
	client *internalent.EntClient,
	worker capability.WorkerInstance,
	evidence capability.CapabilityEvidence,
) {
	t.Helper()
	tuple := evidence.Tuple
	_, err := client.Client().JudgeWorkerCapability.Create().
		SetWorkerInstanceID(worker.ID).SetBootID(worker.BootID).
		SetJudgeSpecProtocol(tuple.JudgeSpecProtocol).SetRuntimeKey(tuple.RuntimeKey).
		SetRuntimeProfileVersion(tuple.RuntimeProfileVersion).
		SetCompilerVersion(tuple.CompilerVersion).
		SetRuntimeBootstrapVersion(tuple.RuntimeBootstrapVersion).
		SetCompileCommandID(tuple.CompileCommandID).SetRunCommandID(tuple.RunCommandID).
		SetImageDigest(tuple.ImageDigest).
		SetManifestVersion(tuple.ManifestVersion).SetManifestChecksum(tuple.ManifestChecksum).
		SetSupervisorProtocol(tuple.SupervisorProtocol).SetCheckerKey(tuple.CheckerKey).
		SetCheckerVersion(tuple.CheckerVersion).SetCapabilityProfileHash(tuple.CapabilityProfileHash).
		SetCheckerImplementationChecksum(evidence.CheckerImplementationChecksum).
		SetSmokeChecksum(evidence.SmokeChecksum).SetGoldenChecksum(evidence.GoldenChecksum).
		SetProbedAt(evidence.ProbedAt).
		SetPassing(true).Save(context.Background())
	if err != nil {
		t.Fatalf("seed capability evidence: %v", err)
	}
}

func assertCapabilityPersistence(
	t *testing.T,
	database *sql.DB,
	worker capability.WorkerInstance,
	wantEnabled bool,
	wantEvidence int,
) {
	t.Helper()
	var enabled bool
	if err := database.QueryRow(`SELECT claims_enabled FROM judge_worker_instances WHERE id = ? AND boot_id = ?`, worker.ID, worker.BootID).Scan(&enabled); err != nil {
		t.Fatalf("read worker claims state: %v", err)
	}
	if enabled != wantEnabled {
		t.Fatalf("claims_enabled = %v, want %v", enabled, wantEnabled)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM judge_worker_capabilities WHERE worker_instance_id = ? AND boot_id = ?`, worker.ID, worker.BootID).Scan(&count); err != nil {
		t.Fatalf("count worker evidence: %v", err)
	}
	if count != wantEvidence {
		t.Fatalf("evidence count = %d, want %d", count, wantEvidence)
	}
}

func openCapabilitySQLite(t *testing.T) (*internalent.EntClient, *sql.DB) {
	t.Helper()
	database, err := sql.Open("sqlite3", "file:"+uuid.NewString()+"?mode=memory&cache=shared&_foreign_keys=1&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("sql.Open(sqlite3) error = %v", err)
	}
	database.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = database.Close() })
	for _, statement := range []string{
		`CREATE TABLE judge_worker_instances (
			id TEXT PRIMARY KEY, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
			worker_key TEXT NOT NULL, boot_id TEXT NOT NULL, boot_started_at DATETIME NOT NULL,
			heartbeat_at DATETIME NOT NULL, claims_enabled BOOLEAN NOT NULL DEFAULT 0,
			total_capacity INTEGER NOT NULL DEFAULT 1,
			free_capacity INTEGER NOT NULL DEFAULT 0,
			UNIQUE (id, boot_id), UNIQUE (boot_id), UNIQUE (worker_key, boot_id)
		)`,
		`CREATE TABLE judge_worker_capabilities (
				id TEXT PRIMARY KEY, boot_id TEXT NOT NULL, judge_spec_protocol TEXT NOT NULL,
				runtime_key TEXT NOT NULL, runtime_profile_version TEXT NOT NULL,
				compiler_version TEXT NOT NULL, runtime_bootstrap_version TEXT NOT NULL,
				compile_command_id TEXT NOT NULL, run_command_id TEXT NOT NULL, image_digest TEXT NOT NULL,
				manifest_version TEXT NOT NULL, manifest_checksum TEXT NOT NULL, supervisor_protocol TEXT NOT NULL,
			checker_key TEXT NOT NULL, checker_version TEXT NOT NULL, capability_profile_hash TEXT NOT NULL,
			checker_implementation_checksum TEXT NOT NULL, smoke_checksum TEXT NOT NULL,
			golden_checksum TEXT NOT NULL, probed_at DATETIME NOT NULL, passing BOOLEAN NOT NULL,
				created_at DATETIME NOT NULL, worker_instance_id TEXT NOT NULL,
				UNIQUE (id, worker_instance_id, boot_id),
				UNIQUE (
					id, worker_instance_id, boot_id, judge_spec_protocol, runtime_key, runtime_profile_version,
					compiler_version, runtime_bootstrap_version, compile_command_id, run_command_id,
					image_digest, manifest_version, manifest_checksum, supervisor_protocol,
					checker_key, checker_version, capability_profile_hash,
					checker_implementation_checksum, smoke_checksum, golden_checksum
				),
				UNIQUE (worker_instance_id, boot_id, capability_profile_hash),
			FOREIGN KEY (worker_instance_id, boot_id) REFERENCES judge_worker_instances (id, boot_id)
		)`,
		`CREATE TABLE activated_judge_capabilities (
			id TEXT PRIMARY KEY, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
				release_id TEXT NOT NULL, approved_release_manifest_checksum TEXT NOT NULL,
				actor_id TEXT NOT NULL, audit_id TEXT NOT NULL, reason TEXT NOT NULL,
				judge_spec_protocol TEXT NOT NULL, runtime_key TEXT NOT NULL, runtime_profile_version TEXT NOT NULL,
				compiler_version TEXT NOT NULL, runtime_bootstrap_version TEXT NOT NULL,
				compile_command_id TEXT NOT NULL, run_command_id TEXT NOT NULL,
				image_digest TEXT NOT NULL, manifest_version TEXT NOT NULL, manifest_checksum TEXT NOT NULL,
			supervisor_protocol TEXT NOT NULL, checker_key TEXT NOT NULL, checker_version TEXT NOT NULL,
			capability_profile_hash TEXT NOT NULL, checker_implementation_checksum TEXT NOT NULL,
			smoke_checksum TEXT NOT NULL, golden_checksum TEXT NOT NULL,
			proof_worker_instance_id TEXT NOT NULL, proof_boot_id TEXT NOT NULL, proof_evidence_id TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT 'active', activated_at DATETIME NOT NULL,
			retired_at DATETIME NULL, retired_by TEXT NULL, retirement_audit_id TEXT NULL, retirement_reason TEXT NULL,
			UNIQUE (release_id, capability_profile_hash),
				FOREIGN KEY (
					proof_evidence_id, proof_worker_instance_id, proof_boot_id,
					judge_spec_protocol, runtime_key, runtime_profile_version,
					compiler_version, runtime_bootstrap_version, compile_command_id, run_command_id,
					image_digest, manifest_version, manifest_checksum, supervisor_protocol,
					checker_key, checker_version, capability_profile_hash,
					checker_implementation_checksum, smoke_checksum, golden_checksum
				) REFERENCES judge_worker_capabilities (
					id, worker_instance_id, boot_id, judge_spec_protocol, runtime_key, runtime_profile_version,
					compiler_version, runtime_bootstrap_version, compile_command_id, run_command_id,
					image_digest, manifest_version, manifest_checksum, supervisor_protocol,
					checker_key, checker_version, capability_profile_hash,
					checker_implementation_checksum, smoke_checksum, golden_checksum
			)
		)`,
		`CREATE TABLE judge_specs (
			id TEXT PRIMARY KEY, submission_id TEXT NOT NULL, generation INTEGER NOT NULL,
			release_id TEXT NOT NULL, approved_release_manifest_checksum TEXT NOT NULL,
			judge_spec_protocol TEXT NOT NULL, runtime_key TEXT NOT NULL,
			runtime_profile_version TEXT NOT NULL, compiler_version TEXT NOT NULL,
			runtime_bootstrap_version TEXT NOT NULL, compile_command_id TEXT NOT NULL,
			run_command_id TEXT NOT NULL, sandbox_image_digest TEXT NOT NULL,
			artifact_manifest_version TEXT NOT NULL, artifact_manifest_checksum TEXT NOT NULL,
			supervisor_protocol_version TEXT NOT NULL, checker_key TEXT NOT NULL,
			checker_version TEXT NOT NULL, checker_implementation_checksum TEXT NOT NULL,
			capability_profile_hash TEXT NOT NULL
		)`,
		`CREATE TABLE judge_jobs (
			id TEXT PRIMARY KEY, submission_id TEXT NOT NULL, generation INTEGER NOT NULL,
			judge_spec_id TEXT NOT NULL, state TEXT NOT NULL
		)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("create capability SQLite schema: %v", err)
		}
	}
	driver := entsql.OpenDB(dialect.SQLite, database)
	return internalent.WrapClientWithDriver(generate.NewClient(generate.Driver(driver)), driver, nil), database
}

func capabilityTestEvidence(t *testing.T, bootID uuid.UUID, probedAt time.Time) capability.CapabilityEvidence {
	t.Helper()
	tuple := capability.CapabilityTuple{
		JudgeSpecProtocol: "judge-spec.v1", RuntimeKey: "cpp17", RuntimeProfileVersion: "2026.07.1",
		CompilerVersion: "gcc-15.2.0", RuntimeBootstrapVersion: "alpine3.23-musl",
		CompileCommandID: "cpp17.compile.v1", RunCommandID: "cpp17.run.v1",
		ImageDigest:        "sha256:aaed1b901a2090c1b63ee5cacd1bc2536f78f741f9a750a85a7f9f9fe1dd6f34",
		ManifestVersion:    "cpp17-artifact.v1",
		ManifestChecksum:   "sha256:b45e743f0c94809cc04fc395c3345cce7749a997393870ba2c37a9de783b41dc",
		SupervisorProtocol: "supervisor.v1", CheckerKey: "exact", CheckerVersion: "v1",
		CheckerImplementationChecksum: "sha256:0ca1bb8dc836e6d75ca2d96e723cd8f0f4de0c1345873842eb3cda599347091a",
	}
	hash, err := capability.ComputeCapabilityProfileHash(tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() error = %v", err)
	}
	tuple.CapabilityProfileHash = hash
	return capability.CapabilityEvidence{
		Tuple: tuple, BootID: bootID,
		CheckerImplementationChecksum: "sha256:0ca1bb8dc836e6d75ca2d96e723cd8f0f4de0c1345873842eb3cda599347091a",
		SmokeChecksum:                 "sha256:c4b00d624e7fa7274056b4f8a3371b45798fd628a5c5e4cedb20ee3d9c5d5ec3",
		GoldenChecksum:                "sha256:f3c7dd28b1ecf7e40c4c695129fd1b67cc65a4149421133446f062886c24dd9d",
		ProbedAt:                      probedAt, Passing: true,
	}
}

func capabilityTestEvidenceVariant(
	t *testing.T,
	bootID uuid.UUID,
	probedAt time.Time,
) capability.CapabilityEvidence {
	t.Helper()
	evidence := capabilityTestEvidence(t, bootID, probedAt)
	evidence.Tuple.RuntimeKey = "python3"
	evidence.Tuple.RuntimeProfileVersion = "2026.07.2"
	evidence.Tuple.CompilerVersion = "cpython-3.12.14"
	evidence.Tuple.RuntimeBootstrapVersion = "cpython-3.12.14"
	evidence.Tuple.CompileCommandID = "python3.compile.v1"
	evidence.Tuple.RunCommandID = "python3.run.v1"
	evidence.Tuple.ManifestVersion = "python3-artifact.v1"
	evidence.Tuple.CapabilityProfileHash = ""
	hash, err := capability.ComputeCapabilityProfileHash(evidence.Tuple)
	if err != nil {
		t.Fatalf("ComputeCapabilityProfileHash() variant error = %v", err)
	}
	evidence.Tuple.CapabilityProfileHash = hash
	return evidence
}
