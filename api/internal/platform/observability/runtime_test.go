package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

func newTestMetrics(t *testing.T) *Metrics {
	t.Helper()
	metrics, err := New(Config{Role: "api", Environment: "test", Version: "test"})
	if err != nil {
		t.Fatalf("New metrics: %v", err)
	}
	return metrics
}

func TestHandlerRejectsUnsupportedMethodWithoutCollecting(t *testing.T) {
	metrics := newTestMetrics(t)
	calls := 0
	if err := metrics.RegisterCollector("method-guard", CollectorFunc(func(context.Context) error {
		calls++
		return nil
	})); err != nil {
		t.Fatalf("RegisterCollector: %v", err)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if calls != 0 {
		t.Fatalf("collector calls = %d, want 0 for rejected method", calls)
	}
}

func TestHandlerCollectsRegisteredRuntimeStateBeforeExposition(t *testing.T) {
	metrics := newTestMetrics(t)
	calls := 0
	if err := metrics.RegisterCollector("judge-state", CollectorFunc(func(ctx context.Context) error {
		calls++
		return metrics.Set(ctx, MetricJudgeOldestReady, 12, nil)
	})); err != nil {
		t.Fatalf("RegisterCollector: %v", err)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if calls != 1 || !strings.Contains(response.Body.String(), "judgify_judge_oldest_ready_job_seconds 12") {
		t.Fatalf("calls=%d body=%q", calls, response.Body.String())
	}
	if err := metrics.RegisterCollector("judge-state", CollectorFunc(func(context.Context) error { return nil })); !errors.Is(err, ErrCollectorExists) {
		t.Fatalf("duplicate collector error = %v, want ErrCollectorExists", err)
	}
}

type nilTestCollector struct{}

func (*nilTestCollector) Collect(context.Context) error { return nil }

func TestRegisterCollectorRejectsTypedNil(t *testing.T) {
	t.Parallel()

	metrics := newTestMetrics(t)
	var collector *nilTestCollector
	if err := metrics.RegisterCollector("typed-nil", collector); !errors.Is(err, ErrInvalidMetrics) {
		t.Fatalf("RegisterCollector typed nil error = %v, want ErrInvalidMetrics", err)
	}
}

func TestObserveDatabaseUsesClosedLowCardinalityClasses(t *testing.T) {
	metrics := newTestMetrics(t)
	metrics.ObserveDatabase(context.Background(), commonent.StatementObservation{
		Class:    commonent.StatementClassRead,
		Duration: 10 * time.Millisecond,
	})
	metrics.ObserveDatabase(context.Background(), commonent.StatementObservation{
		Class:    commonent.StatementClassWrite,
		Duration: 20 * time.Millisecond,
		Err:      context.DeadlineExceeded,
	})
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	metrics.ObserveDatabase(cancelledContext, commonent.StatementObservation{
		Class:    commonent.StatementClassTransaction,
		Duration: time.Millisecond,
		Err:      context.Canceled,
	})

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	body := response.Body.String()
	for _, sample := range []string{
		`query_class="read",outcome="success"`,
		`query_class="write",outcome="timeout"`,
		`query_class="transaction",outcome="cancelled"`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}
