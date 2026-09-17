package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	commonhttp "github.com/huynhanx03/go-common/pkg/common/http"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/di"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// Server is the lifecycle-owned product HTTP server.
type Server struct {
	server *http.Server
	logger *zap.Logger

	mu        sync.Mutex
	started   bool
	serveDone chan struct{}
	serveErr  error
	group     errgroup.Group
}

// NewServer creates a stopped HTTP component.
func NewServer(handler http.Handler, address string, logger *zap.Logger) (*Server, error) {
	if handler == nil || address == "" || logger == nil {
		return nil, fmt.Errorf("http server: invalid dependencies")
	}
	transport := commonhttp.DefaultServerOptions(address)
	return &Server{
		server: &http.Server{
			Addr:              transport.Address,
			Handler:           handler,
			ReadHeaderTimeout: transport.ReadHeaderTimeout,
			ReadTimeout:       transport.ReadTimeout,
			WriteTimeout:      transport.WriteTimeout,
			IdleTimeout:       transport.IdleTimeout,
			MaxHeaderBytes:    transport.MaxHeaderBytes,
		},
		logger:    logger,
		serveDone: make(chan struct{}),
	}, nil
}

// NewHTTPServer creates the API server from an explicit dependency container.
func NewHTTPServer(
	container *di.Container,
	healthRegistry *commonhealth.Registry,
	metrics *observability.Metrics,
	serverConfig settings.Server,
	authenticationConfig settings.Authentication,
	logger *commonlogger.LoggerZap,
) (*Server, error) {
	if container == nil || container.Audit == nil || container.Identity == nil || container.Problem == nil ||
		container.Cultivation == nil || container.Submission == nil ||
		container.Contest == nil || container.Material == nil || container.Realtime == nil ||
		container.Notification == nil ||
		container.NotificationCampaign == nil ||
		container.Operation == nil || container.OperationHTTP == nil ||
		container.OperationRunner == nil || container.Judge == nil ||
		container.DebugLevel == nil || container.DebugWindow == nil ||
		logger == nil || logger.Logger == nil {
		return nil, fmt.Errorf("http server: invalid container")
	}
	authzChecker := middlewares.NewAuthorizationChecker(container.Identity.AuthorizationService)
	routerGroup := NewRouterGroup(
		container.Audit,
		container.Identity.IdentityHandler,
		container.Judge,
		container.Problem.ProblemHandlerGroup,
		container.Cultivation.CultivationHandlerGroup,
		container.Submission.SubmissionHandlerGroup,
		container.Contest.ContestHandlerGroup,
		container.Material.MaterialHandlerGroup,
		container.Notification.Handler,
		container.NotificationCampaign.Handler,
		container.OperationHTTP,
		container.Realtime.Handler,
		container.DebugLevel,
		authzChecker,
		container.Identity.SessionAuthenticator,
	)

	host := os.Getenv("SERVER_HOST")
	if host == "" {
		host = serverConfig.Host
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(serverConfig.Port)
	}
	engine, err := NewEngine(
		routerGroup,
		healthRegistry,
		metrics,
		serverConfig,
		authenticationConfig,
		logger,
	)
	if err != nil {
		return nil, err
	}
	return NewServer(engine, fmt.Sprintf("%s:%s", host, port), logger.Named("http"))
}

func (*Server) Name() string { return "http-server" }

// Run starts serving and returns promptly when the lifecycle is canceled.
// Graceful draining belongs exclusively to Shutdown so the outer lifecycle's
// total and per-component budgets remain authoritative.
func (server *Server) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("http server: nil run context")
	}
	if ctx.Err() != nil {
		return nil
	}
	server.mu.Lock()
	if server.started {
		server.mu.Unlock()
		return fmt.Errorf("http server: already started")
	}
	server.started = true
	runContext := commonlogger.WithContext(ctx, server.logger)
	server.group.Go(func() error {
		commonlogger.FromContext(runContext).Info(
			"HTTP server started",
			zap.String("address", server.server.Addr),
		)
		err := server.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		server.mu.Lock()
		server.serveErr = err
		close(server.serveDone)
		server.mu.Unlock()
		return err
	})
	done := server.serveDone
	server.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil
	case <-done:
		server.mu.Lock()
		err := server.serveErr
		server.mu.Unlock()
		return err
	}
}

// Shutdown gracefully drains HTTP within the exact caller-owned lifecycle
// deadline, then force-closes on timeout.
func (server *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("http server: nil shutdown context")
	}
	server.mu.Lock()
	started := server.started
	done := server.serveDone
	server.mu.Unlock()
	if !started {
		return nil
	}

	shutdownErr := server.server.Shutdown(ctx)
	if shutdownErr != nil && !errors.Is(shutdownErr, http.ErrServerClosed) {
		shutdownErr = errors.Join(shutdownErr, server.server.Close())
	}
	select {
	case <-done:
		server.mu.Lock()
		serveErr := server.serveErr
		server.mu.Unlock()
		shutdownContext := commonlogger.WithContext(ctx, server.logger)
		commonlogger.FromContext(shutdownContext).Info("HTTP server stopped")
		return errors.Join(shutdownErr, serveErr)
	case <-ctx.Done():
		_ = server.server.Close()
		return errors.Join(shutdownErr, ctx.Err())
	}
}
