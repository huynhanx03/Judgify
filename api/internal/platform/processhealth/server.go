package processhealth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
)

// ProbeServer exposes role-local probes on a private listener. API processes
// may route the same handlers through their existing HTTP server; workers use
// this component so readiness is stronger than a PID check.
type ProbeServer struct {
	server *http.Server

	mu       sync.Mutex
	listener net.Listener
	done     chan struct{}
	runErr   error
}

func NewProbeServer(
	address string,
	registry *commonhealth.Registry,
	metrics http.Handler,
) (*ProbeServer, error) {
	if address == "" || registry == nil || metrics == nil {
		return nil, ErrInvalidProcessHealth
	}
	mux := http.NewServeMux()
	healthHandler := commonhealth.HTTPHandler(registry)
	mux.Handle("/live", healthHandler)
	mux.Handle("/ready", healthHandler)
	mux.Handle("/metrics", metrics)
	return &ProbeServer{
		server: &http.Server{
			Addr: address, Handler: mux,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       3 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    8 << 10,
		},
		done: make(chan struct{}),
	}, nil
}

func (*ProbeServer) Name() string { return "private-health-server" }

func (server *ProbeServer) Run(ctx context.Context) error {
	if server == nil || server.server == nil || ctx == nil {
		return ErrInvalidProcessHealth
	}
	listener, err := net.Listen("tcp", server.server.Addr)
	if err != nil {
		return fmt.Errorf("listen for private health: %w", err)
	}
	server.mu.Lock()
	if server.listener != nil {
		server.mu.Unlock()
		_ = listener.Close()
		return ErrInvalidProcessHealth
	}
	server.listener = listener
	server.mu.Unlock()

	err = server.server.Serve(listener)
	server.mu.Lock()
	server.runErr = err
	server.mu.Unlock()
	close(server.done)
	if errors.Is(err, http.ErrServerClosed) || ctx.Err() != nil {
		return nil
	}
	return fmt.Errorf("serve private health: %w", err)
}

func (server *ProbeServer) Shutdown(ctx context.Context) error {
	if server == nil || server.server == nil || ctx == nil {
		return ErrInvalidProcessHealth
	}
	server.mu.Lock()
	listener := server.listener
	server.mu.Unlock()
	if listener == nil {
		return nil
	}
	if err := server.server.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case <-server.done:
		server.mu.Lock()
		runErr := server.runErr
		server.mu.Unlock()
		if errors.Is(runErr, http.ErrServerClosed) {
			return nil
		}
		return runErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
