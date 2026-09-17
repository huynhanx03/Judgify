package infrastructure

import (
	"context"
	"fmt"
	"os"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	grpcserver "github.com/huynhanx03/go-common/pkg/common/grpc/server"
	"github.com/huynhanx03/go-common/pkg/common/lifecycle"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/huynhanx03/judgify/internal/di"
	envoyauthz "github.com/huynhanx03/judgify/internal/infrastructure/authz"
	"github.com/huynhanx03/judgify/internal/infrastructure/bootstrap"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	"github.com/huynhanx03/judgify/internal/platform/processhealth"
	"github.com/huynhanx03/judgify/internal/platform/tracing"
)

// BuildAPI composes API-only dependencies. It never constructs Forge, Docker,
// a judge worker, or starts any component.
func BuildAPI(ctx context.Context, process *bootstrap.Process) ([]lifecycle.Component, error) {
	if ctx == nil || process == nil {
		return nil, fmt.Errorf("compose API: invalid dependencies")
	}
	metrics, err := observability.New(observability.Config{
		Role:        "api",
		Environment: string(process.Config.Server.Mode),
		Version:     process.Config.Observability.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("compose API metrics: %w", err)
	}
	if err := process.RegisterDatabaseObserver(metrics.ObserveDatabase); err != nil {
		return nil, fmt.Errorf("compose API database metrics: %w", err)
	}
	traceProvider, err := tracing.New(ctx, "judgify-api", process.Config.Observability.Version, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		return nil, fmt.Errorf("compose API tracing: %w", err)
	}
	database, err := process.OpenDatabase()
	if err != nil {
		return nil, err
	}
	driver, ok := database.DialectDriver()
	if !ok {
		return nil, fmt.Errorf("compose API database metrics: SQL driver unavailable")
	}
	pool, err := observability.SQLPool(driver)
	if err != nil {
		return nil, fmt.Errorf("compose API database metrics: %w", err)
	}
	if err := metrics.RegisterDatabasePool(pool); err != nil {
		return nil, fmt.Errorf("compose API database pool metrics: %w", err)
	}
	localCache, err := process.OpenCache()
	if err != nil {
		return nil, err
	}
	if err := process.LoadKeys(); err != nil {
		return nil, err
	}

	container, err := di.SetupDependencies(ctx, di.Dependencies{
		Database: database,
		Cache:    localCache,
		Config:   process.Config,
		Logger:   process.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("compose API domains: %w", err)
	}
	if err := container.Cultivation.TraitOfferService.RefreshCatalog(ctx); err != nil {
		return nil, fmt.Errorf("compose onboarding trait catalog: %w", err)
	}
	scheduler, err := NewScheduler(
		container.Problem.ProblemRepo,
		process.Logger.Named("stats-scheduler"),
	)
	if err != nil {
		return nil, err
	}
	eventDelivery, err := NewEventDelivery(
		container,
		database,
		process.Config.Outbox,
		process.Logger,
	)
	if err != nil {
		shutdownAPIComposition(scheduler)
		return nil, err
	}
	authorization, ok := container.Identity.AuthorizationRuntime.(observability.AuthorizationRevisionSource)
	if !ok {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API authorization metrics: runtime unavailable")
	}
	if err := metrics.RegisterDurableAPICollectors(pool, container.OperationRunner.Kinds()); err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API durable metrics: %w", err)
	}
	if err := metrics.RegisterAuthorizationCollector(authorization); err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API authorization metrics: %w", err)
	}
	if err := metrics.RegisterReplayCollector(eventDelivery); err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API replay metrics: %w", err)
	}
	if err := metrics.RegisterWebSocketCollector(container.Realtime.Hub); err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API websocket metrics: %w", err)
	}
	healthRegistry, healthControl, err := NewObservedAPIHealthRegistry(APIHealthDependencies{
		Database:                    process.DatabaseHealthChecker(),
		EventDelivery:               eventDelivery,
		Authorization:               container.Identity.AuthorizationHealth,
		RealtimeHub:                 container.Realtime.HubHealth,
		RealtimeAuthorizationTailer: container.Realtime.AuthorizationTailerHealth,
		SecurityEmail:               container.Identity.SecurityEmailHealth,
		ReceiptJanitor:              container.Identity.ReceiptJanitor,
		OperationRunner:             container.OperationRunner,
		DebugWindow:                 container.DebugWindow,
	}, metrics)
	if err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API health: %w", err)
	}
	probeServer, err := processhealth.NewProbeServer(
		process.Config.Observability.APIProbeAddress,
		healthRegistry,
		metrics.Handler(),
	)
	if err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API private probes: %w", err)
	}
	server, err := NewHTTPServer(
		container,
		healthRegistry,
		metrics,
		process.Config.Server,
		process.Config.Authentication,
		process.Logger,
	)
	if err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, err
	}
	authzTLS, err := envoyauthz.LoadServerMTLS(
		process.Config.Observability.EnvoyAuthzCertificatePath,
		process.Config.Observability.EnvoyAuthzPrivateKeyPath,
		process.Config.Observability.EnvoyAuthzClientCAPath,
	)
	if err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API envoy ext authz TLS: %w", err)
	}
	authzServer, err := grpcserver.New(grpcserver.Config{
		Name:         "envoy-ext-authz-grpc-server",
		Address:      process.Config.Observability.EnvoyAuthzAddress,
		TLS:          authzTLS,
		Logger:       process.Logger.Logger,
		StatsHandler: otelgrpc.NewServerHandler(),
	}, func(server *grpc.Server) {
		authv3.RegisterAuthorizationServer(server, envoyauthz.NewCheckServer(
			envoyauthz.DefaultPolicies(),
			container.Identity.SessionAuthenticator,
			container.Identity.AuthorizationService,
			container.Identity.IdentityHandler.BrowserSession.Access.Name,
		))
	})
	if err != nil {
		shutdownAPIComposition(eventDelivery, scheduler)
		return nil, fmt.Errorf("compose API envoy ext authz: %w", err)
	}
	return orderedAPIComponents(
		container.Contest.Orchestrator,
		scheduler,
		healthControl,
		container.Realtime.TicketStore,
		container.Realtime.HubLifecycle,
		container.Realtime.AuthorizationTailer,
		eventDelivery,
		container.OperationRunner,
		container.Identity.SecurityEmailDispatcher,
		container.Identity.ReceiptJanitor,
		container.Identity.AuthorizationRuntime,
		container.Identity.SessionAuthenticator,
		container.DebugWindow,
		probeServer,
		traceProvider,
		authzServer,
		server,
	), nil
}

const apiCompositionCleanupTimeout = 5 * time.Second

// shutdownAPIComposition owns only inert components constructed before the
// lifecycle starts. The timeout keeps a future adapter regression from
// turning a composition error into an unbounded startup hang.
func shutdownAPIComposition(components ...lifecycle.Component) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		apiCompositionCleanupTimeout,
	)
	defer cancel()
	for _, component := range components {
		if component != nil {
			_ = component.Shutdown(ctx)
		}
	}
}

// orderedAPIComponents expresses admission dependencies directly. The shared
// lifecycle shuts components down in reverse registration order, so the HTTP
// server stops accepting requests before the session touch writer drains.
func orderedAPIComponents(
	orchestrator lifecycle.Component,
	scheduler lifecycle.Component,
	healthControl lifecycle.Component,
	realtimeTickets lifecycle.Component,
	realtimeHub lifecycle.Component,
	realtimeAuthorizationTailer lifecycle.Component,
	eventDelivery lifecycle.Component,
	operationRunner lifecycle.Component,
	securityEmail lifecycle.Component,
	receiptJanitor lifecycle.Component,
	authorizationRuntime lifecycle.Component,
	sessionAuthenticator lifecycle.Component,
	debugWindow lifecycle.Component,
	probeServer lifecycle.Component,
	traceProvider lifecycle.Component,
	authzServer lifecycle.Component,
	server lifecycle.Component,
) []lifecycle.Component {
	return []lifecycle.Component{
		orchestrator,
		scheduler,
		healthControl,
		realtimeTickets,
		realtimeHub,
		realtimeAuthorizationTailer,
		eventDelivery,
		operationRunner,
		securityEmail,
		receiptJanitor,
		authorizationRuntime,
		sessionAuthenticator,
		debugWindow,
		probeServer,
		traceProvider,
		authzServer,
		server,
	}
}
