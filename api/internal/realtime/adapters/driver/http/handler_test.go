package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/internal/realtime"
)

func TestIssueTicketMapsCapacityAndNeverReturnsSecretInError(t *testing.T) {
	t.Parallel()
	store, err := realtime.NewTicketStore(realtime.TicketConfig{
		TTL:                    time.Minute,
		CleanupInterval:        time.Minute,
		Capacity:               1,
		MaxTicketsPerPrincipal: 1,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerGroup(store, nethttp.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	principal := validRealtimePrincipal()
	ctx := authentication.WithPrincipal(context.Background(), principal)
	issued, err := handler.IssueTicket(ctx)
	if err != nil || issued == nil || issued.Ticket == "" {
		t.Fatalf("IssueTicket() = %#v, %v", issued, err)
	}
	_, err = handler.IssueTicket(ctx)
	var appError *apperr.AppError
	if !errors.As(err, &appError) || appError.Code != apperr.CodeTooManyRequests {
		t.Fatalf("capacity error = %v", err)
	}
	if appError.Error() == issued.Ticket {
		t.Fatal("ticket secret rendered in error")
	}
}

func TestNoStoreAndWebSocketDelegation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := realtime.NewTicketStore(realtime.DefaultTicketConfig(), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler, err := NewHandlerGroup(store, nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, _ *nethttp.Request) {
		called = true
		writer.WriteHeader(nethttp.StatusSwitchingProtocols)
	}))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/ticket", NoStore, func(c *gin.Context) { c.Status(nethttp.StatusNoContent) })
	router.GET("/ws", handler.WebSocket)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(nethttp.MethodGet, "/ticket", nil))
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("ticket cache headers = %#v", recorder.Header())
	}
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(nethttp.MethodGet, "/ws", nil))
	if !called || recorder.Code != nethttp.StatusSwitchingProtocols {
		t.Fatalf("websocket delegated=%t status=%d", called, recorder.Code)
	}
}

func validRealtimePrincipal() authentication.Principal {
	now := time.Now().UTC()
	userID := uuid.NewString()
	return authentication.Principal{
		Authenticated:      true,
		Subject:            userID,
		UserID:             userID,
		SessionID:          uuid.NewString(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(time.Hour),
	}
}
