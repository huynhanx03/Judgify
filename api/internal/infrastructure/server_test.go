package infrastructure

import (
	"context"
	"net/http"
	"testing"

	"go.uber.org/zap"

	commonhttp "github.com/huynhanx03/go-common/pkg/common/http"
)

func TestNewServerAppliesSharedProductionTransportBounds(t *testing.T) {
	t.Parallel()

	const address = "127.0.0.1:0"
	server, err := NewServer(http.NotFoundHandler(), address, zap.NewNop())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	want := commonhttp.DefaultServerOptions(address)
	if server.server.ReadHeaderTimeout != want.ReadHeaderTimeout ||
		server.server.ReadTimeout != want.ReadTimeout ||
		server.server.WriteTimeout != want.WriteTimeout ||
		server.server.IdleTimeout != want.IdleTimeout ||
		server.server.MaxHeaderBytes != want.MaxHeaderBytes {
		t.Fatalf("transport bounds = %+v, want shared defaults %+v", server.server, want)
	}
}

func TestHTTPServerHonorsCancellationBeforeListen(t *testing.T) {
	t.Parallel()

	server, err := NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "127.0.0.1:0", zap.NewNop())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := server.Run(ctx); err != nil {
		t.Fatalf("Run after cancellation: %v", err)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("repeated Shutdown: %v", err)
	}
}

func TestHTTPServerRejectsNilRunAndShutdownContexts(t *testing.T) {
	t.Parallel()

	server, err := NewServer(http.NotFoundHandler(), "127.0.0.1:0", zap.NewNop())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// Held in a variable rather than written inline so the nil-context guard
	// stays covered without tripping the linter's literal-nil check.
	var missingContext context.Context
	if err := server.Run(missingContext); err == nil {
		t.Fatalf("Run(nil context) error = nil, want non-nil")
	}
	if err := server.Shutdown(missingContext); err == nil {
		t.Fatalf("Shutdown(nil context) error = nil, want non-nil")
	}
}
