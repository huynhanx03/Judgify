package infrastructure

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	commonlogger "github.com/huynhanx03/judgify/internal/platform/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerContextKeepsIdentityOutOfAmbientLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.DebugLevel)
	root := zap.New(core)
	now := time.Now().UTC()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            "00000000-0000-7000-8000-000000000001",
		UserID:             "00000000-0000-7000-8000-000000000001",
		SessionID:          "00000000-0000-7000-8000-000000000002",
		TokenID:            "00000000-0000-7000-8000-000000000003",
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(time.Hour),
	}

	router := gin.New()
	router.Use(middlewares.RequestID())
	router.Use(requestLoggerContext(root, false))
	router.Use(func(c *gin.Context) {
		ctx := authentication.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.GET("/resource", func(c *gin.Context) {
		commonlogger.FromContext(c.Request.Context()).Info("domain operation")
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	entries := logs.FilterMessage("domain operation").All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["correlation_id"] == "" {
		t.Fatal("correlation identity missing from request log")
	}
	if _, exists := fields["user_id"]; exists {
		t.Fatal("user identity was attached to ambient logs")
	}
	if _, exists := fields["session_id"]; exists {
		t.Fatal("session identity was attached to ambient logs")
	}
	if _, exists := fields["token_id"]; exists {
		t.Fatal("token identity was attached to ambient logs")
	}
	if _, exists := fields["token"]; exists {
		t.Fatal("raw token was attached to ambient logs")
	}
}
