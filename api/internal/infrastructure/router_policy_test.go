package infrastructure

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/settings"
)

func TestInstallRequestPolicyRejectsOversizedBodiesBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := installRequestPolicy(engine, settings.Server{
		MaxRequestBodyBytes: 8, RequestTimeoutSeconds: 1,
	}); err != nil {
		t.Fatalf("installRequestPolicy: %v", err)
	}
	var called atomic.Bool
	engine.POST("/body", func(context *gin.Context) {
		called.Store(true)
		context.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/body", bytes.NewReader([]byte("too-large")))
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", recorder.Code, recorder.Body.String())
	}
	if called.Load() {
		t.Fatal("oversized request reached handler")
	}
}

func TestInstallRequestPolicyBoundsHandlerContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := installRequestPolicy(engine, settings.Server{
		MaxRequestBodyBytes: 1024, RequestTimeoutSeconds: 1,
	}); err != nil {
		t.Fatalf("installRequestPolicy: %v", err)
	}
	engine.GET("/slow", func(context *gin.Context) {
		<-context.Request.Context().Done()
	})

	started := time.Now()
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/slow", nil))
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504; body=%s", recorder.Code, recorder.Body.String())
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("request deadline elapsed = %s, want bounded", elapsed)
	}
}

func TestRequestPolicyDoesNotShareInboundCircuitStateAcrossRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := installRequestPolicy(engine, settings.Server{
		MaxRequestBodyBytes: 1024, RequestTimeoutSeconds: 1,
	}); err != nil {
		t.Fatalf("installRequestPolicy: %v", err)
	}
	var calls atomic.Int32
	engine.GET("/failure", func(context *gin.Context) {
		calls.Add(1)
		context.Status(http.StatusInternalServerError)
	})
	for range 7 {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/failure", nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want handler's 500", recorder.Code)
		}
	}
	if got := calls.Load(); got != 7 {
		t.Fatalf("handler calls = %d, want every independent inbound request", got)
	}
}
