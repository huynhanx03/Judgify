package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"

	"github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/judge"
	judgedb "github.com/huynhanx03/judgify/internal/judge/adapters/driven/db"
	judgeassets "github.com/huynhanx03/judgify/internal/judge/assets"
	"github.com/huynhanx03/judgify/internal/judge/capability"
	"github.com/huynhanx03/judgify/internal/judge/executor"
	"github.com/huynhanx03/judgify/internal/judge/supervisor"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	problemdb "github.com/huynhanx03/judgify/internal/problem/adapters/driven/db"
	submissiondb "github.com/huynhanx03/judgify/internal/submission/adapters/driven/db"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
	"github.com/huynhanx03/judgify/pkg/judging"
	"github.com/huynhanx03/judgify/pkg/settings"
)

const (
	productionStagerGlobalBytes  = 1 << 30
	productionStagerGlobalInodes = 4096
	productionStagerAttemptBytes = 64 << 20
	productionStagerAttemptNodes = 512
)

var productionImageDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// productionRuntime owns durable judging. PostgreSQL owns judge correctness
// and leases; the sibling worker-owned reward-delivery component owns the
// local Forge bridge without making Forge authoritative for judge work.
type productionRuntime struct {
	config   settings.Judge
	database *ent.EntClient
	logger   *zap.Logger

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}

	worker         capability.WorkerInstance
	registry       *capability.Registry
	durable        *judge.DurableWorker
	supervisor     *supervisor.SandboxSupervisor
	stager         *executor.Stager
	readiness      *RuntimeReadinessState
	metricObserver judgeMetricsObserver
}

func newProductionRuntime(
	config settings.Judge,
	database *ent.EntClient,
	logger *zap.Logger,
	metrics *observability.Metrics,
) (*productionRuntime, error) {
	config = normalizeJudgeConfig(config)
	if err := validateProductionJudgeConfig(config); err != nil || database == nil || logger == nil || metrics == nil {
		return nil, errors.Join(ErrInvalidWorker, err)
	}
	readiness, err := NewRuntimeReadinessState(
		config.WorkerHeartbeatFreshness,
		config.MinimumFreeDiskBytes,
	)
	if err != nil {
		return nil, errors.Join(ErrInvalidWorker, err)
	}
	return &productionRuntime{
		config: config, database: database,
		logger: logger, done: make(chan struct{}), readiness: readiness,
		metricObserver: judgeMetricsObserver{metrics: metrics},
	}, nil
}

func (runtime *productionRuntime) Name() string { return "judge-worker" }

func (runtime *productionRuntime) WorkerMetrics(
	ctx context.Context,
) (observability.WorkerRuntimeSnapshot, error) {
	if runtime == nil || ctx == nil {
		return observability.WorkerRuntimeSnapshot{}, ErrInvalidWorker
	}
	if err := ctx.Err(); err != nil {
		return observability.WorkerRuntimeSnapshot{}, err
	}
	state := runtime.readiness.Snapshot()
	snapshot := observability.WorkerRuntimeSnapshot{
		TotalCapacity: runtime.config.WorkerCount,
		ClaimsEnabled: state.ClaimsEnabled,
	}
	if state.CapacityProbeBound && !state.Draining {
		capacity, err := runtime.readiness.CapacitySnapshot(ctx)
		if err != nil {
			return observability.WorkerRuntimeSnapshot{}, err
		}
		snapshot.FreeCapacity = capacity.FreeSandboxes
		snapshot.DiskAvailableBytes = capacity.DiskAvailableBytes
	}
	runtime.mu.Lock()
	sandbox := runtime.supervisor
	runtime.mu.Unlock()
	if sandbox != nil {
		accounting := sandbox.Metrics()
		snapshot.SandboxCPUSeconds = accounting.CPUUsage.Seconds()
		snapshot.SandboxMemoryCurrentBytes = accounting.MemoryCurrentBytes
		snapshot.SandboxOOMEvents = accounting.OOMEvents
		snapshot.SandboxOOMKills = accounting.OOMKills
		snapshot.SandboxCPUThrottledEvents = accounting.CPUThrottledEvents
		snapshot.SandboxPIDLimitEvents = accounting.PIDLimitEvents
	}
	return snapshot, nil
}

func (runtime *productionRuntime) Run(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return ErrInvalidWorker
	}
	runtime.mu.Lock()
	if runtime.started {
		runtime.mu.Unlock()
		return ErrInvalidWorker
	}
	runCtx, cancel := context.WithCancel(ctx)
	runtime.started, runtime.cancel = true, cancel
	runtime.mu.Unlock()
	defer close(runtime.done)
	defer cancel()
	defer runtime.readiness.BeginDrain()

	if err := runtime.initialize(runCtx); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		return errors.Join(err, runtime.disableClaims(cleanupCtx), runtime.shutdownOwned(cleanupCtx))
	}
	err := runtime.durable.Run(runCtx)
	if ctx.Err() != nil {
		return nil
	}
	return errors.Join(errors.New("judge worker component stopped"), err)
}

func (runtime *productionRuntime) initialize(ctx context.Context) error {
	catalog, err := loadReleaseCatalog(ctx, runtime.config.ReleaseID, runtime.config.ReleaseManifestPath)
	if err != nil {
		return fmt.Errorf("load approved runtime catalog: %w", err)
	}
	if catalog.ReleaseID() != runtime.config.ReleaseID || len(catalog.ApprovedTuples()) == 0 {
		return ErrInvalidWorker
	}
	tuples := catalog.ApprovedTuples()
	configuredImageDigest, ok := configuredSandboxDigest(runtime.config.DockerImage)
	if !ok {
		return ErrInvalidWorker
	}
	for _, tuple := range tuples {
		if tuple.ImageDigest != configuredImageDigest {
			return errors.New("configured sandbox image does not match approved release")
		}
	}
	installer, err := problemdb.NewReleaseProfileInstaller(runtime.database)
	if err != nil {
		return err
	}
	if err := installer.InstallAndVerify(ctx, catalog.RuntimeProfiles(), catalog.CheckerProfiles()); err != nil {
		return err
	}

	worker, err := newProductionWorkerInstance(runtime.config, time.Now().UTC())
	if err != nil {
		return err
	}
	evidenceStore, err := judgedb.NewCapabilityEvidenceStore(runtime.database)
	if err != nil {
		return err
	}
	registry, err := capability.NewRegistry(
		catalog,
		evidenceStore,
		runtime.config.WorkerHeartbeatFreshness,
		tuples,
	)
	if err != nil {
		return err
	}
	if err := registry.RegisterBoot(ctx, worker); err != nil {
		return fmt.Errorf("register claims-off worker boot: %w", err)
	}
	runtime.mu.Lock()
	runtime.worker, runtime.registry = worker, registry
	runtime.mu.Unlock()
	if err := runtime.readiness.BindCurrentBoot(worker, tuples); err != nil {
		return err
	}

	engine, err := supervisor.NewDockerEngine(supervisor.DockerEngineConfig{SocketPath: runtime.config.DockerSocketPath})
	if err != nil {
		return err
	}
	probe, err := capability.NewProductionProbe(runtime.config.DockerSocketPath)
	if err != nil {
		return err
	}
	sandbox, err := supervisor.NewSandboxSupervisor(supervisor.Config{
		Engine: engine, Cgroups: supervisor.NewProductionCgroupV2Resolver(), Probe: probe,
		CapabilityTuple: tuples[0], BootID: worker.BootID,
	})
	if err != nil {
		return err
	}
	runtime.mu.Lock()
	runtime.supervisor = sandbox
	runtime.mu.Unlock()
	stager, err := executor.NewStager(executor.StagerConfig{
		Root: runtime.config.StagingRoot, GlobalBytes: productionStagerGlobalBytes,
		GlobalInodes: productionStagerGlobalInodes, PerAttemptBytes: productionStagerAttemptBytes,
		PerAttemptInodes: productionStagerAttemptNodes, MinimumFreeBytes: int64(runtime.config.MinimumFreeDiskBytes),
	})
	if err != nil {
		return err
	}
	runtime.mu.Lock()
	runtime.stager = stager
	runtime.mu.Unlock()
	scheduler, err := judge.NewFairScheduler(judge.FairSchedulerConfig{
		Capacity: min(runtime.config.MaximumReadyBacklog, 4096), Concurrency: runtime.config.WorkerCount,
	})
	if err != nil {
		return err
	}
	capacity, err := judge.NewHostCapacityProbe(scheduler, runtime.config.AdmissionDiskPath)
	if err != nil {
		return err
	}
	if err := runtime.readiness.BindCapacityProbe(capacity); err != nil {
		return err
	}
	jobs, err := submissiondb.NewJobRepositoryFromEnt(runtime.database, submissiondb.JobRepositoryConfig{HeartbeatFreshness: runtime.config.WorkerHeartbeatFreshness})
	if err != nil {
		return err
	}
	jobService, err := submissionservice.NewJobService(jobs, scheduler, capacity, submissionservice.JobServiceConfig{
		WorkerID: worker.ID, WorkerBootID: worker.BootID, LeaseOwner: worker.WorkerKey,
		LeaseDuration: runtime.config.LeaseDuration, HeartbeatFreshness: runtime.config.WorkerHeartbeatFreshness,
		GlobalRunningLimit: runtime.config.PoolSize, PerUserRunningLimit: runtime.config.MaximumActiveSubmissionsPerUser,
		MinimumDiskBytes: int64(runtime.config.MinimumFreeDiskBytes), MaximumLocalDepth: min(runtime.config.MaximumReadyBacklog, 4096),
		MaximumOldestAge: runtime.config.MaximumOldestReadyAge, ReapLimit: 256,
		RetryInitial: runtime.config.LeaseRetryFloor, RetryMaximum: 30 * time.Second,
		Observer: runtime.metricObserver,
	})
	if err != nil {
		return err
	}
	compiler, err := executor.NewCompiler(sandbox, stager)
	if err != nil {
		return err
	}
	testcases, err := executor.NewTestcaseExecutor(sandbox)
	if err != nil {
		return err
	}
	finalizerRepository, err := submissiondb.NewFinalizationRepositoryFromEnt(runtime.database)
	if err != nil {
		return err
	}
	finalizer, err := submissionservice.NewObservedFinalizationService(
		finalizerRepository,
		runtime.metricObserver,
	)
	if err != nil {
		return err
	}
	leaseExecutor, err := judge.NewDurableLeaseExecutor(judge.LeaseExecutorDependencies{
		Plans: jobs, Cases: jobs, Catalog: catalog, Jobs: jobService, Compiler: compiler,
		Testcases: testcases, Assets: judgeassets.Decoder{}, Finalizer: finalizer,
		Artifacts: judge.StagedArtifactReleaser{},
	}, judge.LeaseExecutorConfig{
		MaximumSourceBytes: judging.MaximumSourceCodeBytes, FileBytes: productionStagerAttemptBytes,
		TmpfsBytes:      int64(runtime.config.SandboxTmpfsSizeMb) << 20,
		DiagnosticBytes: int64(runtime.config.MaximumDiagnosticBytes), OpenFiles: 64, CleanupTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	durable, err := judge.NewDurableWorker(jobService, scheduler, leaseExecutor, judge.DurableWorkerConfig{
		ClaimInterval: runtime.config.LeaseRetryFloor, ReapInterval: runtime.config.LeaseHeartbeat,
		WorkerHeartbeatInterval: runtime.config.LeaseHeartbeat, LeaseHeartbeatInterval: runtime.config.LeaseHeartbeat,
	}, runtime.logger.Named("durable"))
	if err != nil {
		return err
	}
	runtime.mu.Lock()
	runtime.durable = durable
	runtime.mu.Unlock()

	advertisements := make([]capability.CapabilityAdvertisement, 0, len(tuples))
	for _, tuple := range tuples {
		evidence, probeErr := probe.Probe(ctx, tuple, worker.BootID)
		if probeErr != nil {
			return fmt.Errorf("probe approved capability %s: %w", tuple.CapabilityProfileHash, probeErr)
		}
		if err := runtime.readiness.RecordPassingProbe(evidence); err != nil {
			return fmt.Errorf("record worker capability readiness: %w", err)
		}
		advertisements = append(
			advertisements,
			capability.CapabilityAdvertisement{Evidence: evidence},
		)
	}
	if err := runtime.readiness.MarkInitialized(); err != nil {
		return err
	}
	if err := registry.ActivateSet(
		ctx,
		worker,
		advertisements,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("activate exact worker capability set: %w", err)
	}
	if err := runtime.readiness.MarkClaimsEnabled(); err != nil {
		return fmt.Errorf("record claims-enabled readiness: %w", err)
	}
	return nil
}

func (runtime *productionRuntime) Shutdown(ctx context.Context) error {
	if runtime == nil || ctx == nil {
		return ErrInvalidWorker
	}
	runtime.mu.Lock()
	started, cancel := runtime.started, runtime.cancel
	runtime.mu.Unlock()
	if !started {
		return nil
	}
	claimsErr := runtime.disableClaims(ctx)
	if cancel != nil {
		cancel()
	}
	select {
	case <-runtime.done:
		return errors.Join(claimsErr, runtime.shutdownOwned(ctx))
	case <-ctx.Done():
		return errors.Join(claimsErr, ctx.Err())
	}
}

func (runtime *productionRuntime) disableClaims(ctx context.Context) error {
	if runtime != nil && runtime.readiness != nil {
		runtime.readiness.BeginDrain()
	}
	runtime.mu.Lock()
	registry, worker := runtime.registry, runtime.worker
	runtime.mu.Unlock()
	if registry == nil || worker.ID == uuid.Nil {
		return nil
	}
	// Registry owns the durable EvidenceStore and keeps the lifecycle operation
	// fail-closed. The store call is intentionally performed before cancellation.
	return registry.DisableClaims(ctx, worker.ID, worker.BootID)
}

func (runtime *productionRuntime) shutdownOwned(ctx context.Context) error {
	runtime.mu.Lock()
	sandbox, stager := runtime.supervisor, runtime.stager
	runtime.mu.Unlock()
	var failures []error
	if sandbox != nil {
		failures = append(failures, sandbox.Shutdown(ctx))
	}
	if stager != nil {
		failures = append(failures, stager.Close())
	}
	return errors.Join(failures...)
}

func newProductionWorkerInstance(config settings.Judge, now time.Time) (capability.WorkerInstance, error) {
	workerID, err := uuid.NewV7()
	if err != nil {
		return capability.WorkerInstance{}, err
	}
	bootID, err := uuid.NewV7()
	if err != nil {
		return capability.WorkerInstance{}, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return capability.WorkerInstance{}, err
	}
	key := canonicalWorkerKey(config.WorkerIdentityPrefix, hostname, os.Getpid())
	if key == "" {
		return capability.WorkerInstance{}, ErrInvalidWorker
	}
	worker := capability.NewWorkerInstance(workerID, key, bootID, now)
	worker.TotalCapacity = config.WorkerCount
	worker.FreeCapacity = 0
	return worker, nil
}

func canonicalWorkerKey(prefix, hostname string, pid int) string {
	value := strings.ToLower(strings.TrimSpace(prefix) + "-" + strings.TrimSpace(hostname) + "-" + fmt.Sprint(pid))
	value = strings.Map(func(r rune) rune {
		if unicode.IsLower(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-._")
	if len(value) == 0 || len(value) > 128 {
		return ""
	}
	return value
}

func validateProductionJudgeConfig(config settings.Judge) error {
	if err := validateJudgeContentLimits(config); err != nil {
		return err
	}
	if config.ReleaseID == "" || validateReleaseManifestPath(config.ReleaseManifestPath) != nil ||
		configuredSandboxImageInvalid(config.DockerImage) ||
		config.DockerSocketPath == "" || config.StagingRoot == "" || config.WorkerCount <= 0 ||
		config.PoolSize <= 0 || config.MaximumReadyBacklog <= 0 || config.MaximumOldestReadyAge <= 0 ||
		config.MaximumActiveSubmissionsPerUser <= 0 || config.WorkerHeartbeatFreshness <= 0 ||
		config.MinimumFreeDiskBytes == 0 ||
		config.MinimumFreeDiskBytes > uint64(^uint64(0)>>1) ||
		config.AdmissionDiskPath == "" ||
		config.LeaseDuration <= 0 || config.LeaseHeartbeat <= 0 ||
		config.LeaseHeartbeat > config.LeaseDuration/4 ||
		config.LeaseHeartbeat > config.WorkerHeartbeatFreshness/2 ||
		config.LeaseRetryFloor <= 0 || config.SandboxTmpfsSizeMb <= 0 {
		return ErrInvalidWorker
	}
	return nil
}

func configuredSandboxImageInvalid(reference string) bool {
	_, ok := configuredSandboxDigest(reference)
	return !ok
}

func configuredSandboxDigest(reference string) (string, bool) {
	value := strings.TrimSpace(reference)
	digest := value
	if marker := strings.LastIndex(value, "@sha256:"); marker >= 0 {
		if marker == 0 || strings.ContainsAny(value[:marker], " \t\r\n") {
			return "", false
		}
		digest = value[marker+1:]
	} else if !strings.HasPrefix(value, "sha256:") {
		return "", false
	}
	if !productionImageDigestPattern.MatchString(digest) {
		return "", false
	}
	body := strings.TrimPrefix(digest, "sha256:")
	for index := 1; index < len(body); index++ {
		if body[index] != body[0] {
			return digest, true
		}
	}
	return "", false
}

var _ lifecycle.Component = (*productionRuntime)(nil)
