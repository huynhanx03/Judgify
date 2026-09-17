// Package worker owns the production judge-worker process graph. Product HTTP
// handlers never import this package, Docker, or Forge.
package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"entgo.io/ent/dialect"

	"github.com/huynhanx03/go-common/pkg/common/lifecycle"

	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	"github.com/huynhanx03/judgify/internal/platform/processhealth"
	"github.com/huynhanx03/judgify/internal/platform/tracing"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/pkg/judging"
	"github.com/huynhanx03/judgify/pkg/settings"
)

var ErrInvalidWorker = errors.New("worker runtime: invalid configuration")

// Build composes the worker runtime and its private process-health surface. It
// opens only the shared database; all returned components remain inert until
// the process lifecycle starts them.
func Build(ctx context.Context, process *bootstrap.Process) ([]lifecycle.Component, error) {
	if ctx == nil || process == nil {
		return nil, ErrInvalidWorker
	}
	traceProvider, err := tracing.New(ctx, "judgify-worker", process.Config.Observability.Version, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		return nil, fmt.Errorf("compose worker tracing: %w", err)
	}
	metrics, err := observability.New(observability.Config{
		Role:        string(processhealth.RoleWorker),
		Environment: string(process.Config.Server.Mode),
		Version:     process.Config.Observability.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("compose worker metrics: %w", err)
	}
	if err := process.RegisterDatabaseObserver(metrics.ObserveDatabase); err != nil {
		return nil, fmt.Errorf("compose worker database metrics: %w", err)
	}
	database, err := process.OpenDatabase()
	if err != nil {
		return nil, err
	}
	driver, ok := database.DialectDriver()
	if !ok || driver.Dialect() != dialect.Postgres {
		return nil, fmt.Errorf("%w: worker SQL driver unavailable", ErrInvalidWorker)
	}
	pool, err := observability.SQLPool(driver)
	if err != nil {
		return nil, fmt.Errorf("%w: worker SQL pool unavailable", ErrInvalidWorker)
	}
	if err := metrics.RegisterDatabasePool(pool); err != nil {
		return nil, fmt.Errorf("compose worker database pool metrics: %w", err)
	}
	if err := metrics.RegisterJudgeQueueCollector(pool); err != nil {
		return nil, fmt.Errorf("compose worker judge queue metrics: %w", err)
	}
	if err := metrics.RegisterSolveRewardDeliveryCollectors(pool); err != nil {
		return nil, fmt.Errorf("compose solve reward durable metrics: %w", err)
	}
	runtime, err := newProductionRuntime(
		process.Config.Judge,
		database,
		process.Logger.Named("judge-worker"),
		metrics,
	)
	if err != nil {
		return nil, err
	}
	if err := metrics.RegisterWorkerRuntimeCollector(runtime); err != nil {
		return nil, fmt.Errorf("compose worker runtime metrics: %w", err)
	}
	rewardDelivery, err := newRewardDelivery(
		process.Config.Forge,
		process.Config.Outbox,
		database,
		process.Logger.Named("solve-reward-delivery"),
	)
	if err != nil {
		return nil, fmt.Errorf("compose solve reward delivery: %w", err)
	}
	if err := metrics.RegisterForgeRuntimeCollector(rewardDelivery); err != nil {
		return nil, fmt.Errorf("compose solve reward Forge metrics: %w", err)
	}
	readiness, _, err := NewWorkerReadinessDependencies(
		runtime.readiness,
		pool,
		process.DatabaseHealthChecker(),
		rewardDelivery,
	)
	if err != nil {
		return nil, fmt.Errorf("compose worker readiness: %w", err)
	}
	healthDependencies, err := WorkerHealthDependencies(readiness)
	if err != nil {
		return nil, fmt.Errorf("compose worker health dependencies: %w", err)
	}
	healthRegistry, healthControl, err := processhealth.New(
		processhealth.RoleWorker,
		healthDependencies,
		metrics,
	)
	if err != nil {
		return nil, fmt.Errorf("compose worker health: %w", err)
	}
	probeServer, err := processhealth.NewProbeServer(
		process.Config.Observability.WorkerProbeAddress,
		healthRegistry,
		metrics.Handler(),
	)
	if err != nil {
		return nil, fmt.Errorf("compose worker private probes: %w", err)
	}
	return []lifecycle.Component{
		traceProvider,
		healthControl,
		runtime,
		rewardDelivery,
		workerProbeServer{next: probeServer},
	}, nil
}

// workerProbeServer gives the worker listener a role-specific lifecycle name.
// This keeps the development-only all role from colliding with the API's
// private probe component while preserving the shared server implementation.
type workerProbeServer struct {
	next *processhealth.ProbeServer
}

func (workerProbeServer) Name() string { return "judge-worker-private-health-server" }

func (server workerProbeServer) Run(ctx context.Context) error {
	if server.next == nil {
		return ErrInvalidWorker
	}
	return server.next.Run(ctx)
}

func (server workerProbeServer) Shutdown(ctx context.Context) error {
	if server.next == nil {
		return ErrInvalidWorker
	}
	return server.next.Shutdown(ctx)
}

// The authoring API and database accept exactly these canonical maxima. A
// smaller worker value would reject valid persisted problems; a larger value
// would make enforcement depend on process-local configuration.
func validateJudgeContentLimits(config settings.Judge) error {
	if config.MaximumTestInputBytes != judging.MaximumTestInputBytes ||
		config.MaximumOutputBytes != problemcontract.MaximumOutputLimitBytes {
		return fmt.Errorf(
			"%w: content byte limits input=%d output=%d, want input=%d output=%d",
			ErrInvalidWorker,
			config.MaximumTestInputBytes,
			config.MaximumOutputBytes,
			judging.MaximumTestInputBytes,
			problemcontract.MaximumOutputLimitBytes,
		)
	}
	return nil
}

func normalizeJudgeConfig(config settings.Judge) settings.Judge {
	// Contestant code never receives network access. An omitted value remains
	// fail-closed instead of becoming SSRF or exfiltration capability.
	config.NetworkDisabled = true
	if config.ContainerMemoryMb <= 0 {
		config.ContainerMemoryMb = problemcontract.MaximumMemoryLimitKilobytes / 1024
	}
	if config.SandboxTmpfsSizeMb <= 0 {
		config.SandboxTmpfsSizeMb = 64
	}
	if config.MaximumTestInputBytes <= 0 {
		config.MaximumTestInputBytes = judging.MaximumTestInputBytes
	}
	if config.MaximumOutputBytes <= 0 {
		config.MaximumOutputBytes = problemcontract.MaximumOutputLimitBytes
	}
	if config.MaximumDiagnosticBytes <= 0 {
		config.MaximumDiagnosticBytes = submissionentity.MaximumJudgeDiagnosticBytes
	}
	if config.WorkerIdentityPrefix == "" {
		config.WorkerIdentityPrefix = "judgify-judge"
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 30 * time.Second
	}
	if config.LeaseHeartbeat == 0 {
		config.LeaseHeartbeat = config.LeaseDuration / 5
	}
	if config.LeaseRetryFloor == 0 {
		config.LeaseRetryFloor = 100 * time.Millisecond
	}
	if config.MaximumAttempts == 0 {
		config.MaximumAttempts = 5
	}
	return config
}
