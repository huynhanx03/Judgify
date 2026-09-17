package processhealth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	platformmetrics "github.com/huynhanx03/judgify/internal/platform/observability"
)

type typedNilHealthChecker struct{}

func (*typedNilHealthChecker) Check(context.Context) error { return nil }

func TestNewRejectsTypedNilDependencyBeforeMetricsDecoration(t *testing.T) {
	metrics, err := platformmetrics.New(platformmetrics.Config{
		Role: "api", Environment: "test", Version: "test",
	})
	if err != nil {
		t.Fatalf("New metrics: %v", err)
	}
	var checker *typedNilHealthChecker

	registry, control, err := New(RoleAPI, []Dependency{
		{
			Name: "typed-nil", Checker: checker,
			Probes: commonhealth.ProbeReadiness, Critical: true,
			Timeout: time.Second, ErrorCode: "typed_nil",
		},
	}, metrics)
	if !errors.Is(err, ErrInvalidProcessHealth) {
		t.Fatalf("New typed nil error = %v, want ErrInvalidProcessHealth", err)
	}
	if registry != nil || control != nil {
		t.Fatal("New returned partial process-health state for typed nil checker")
	}
}

func TestObservedCheckerClassifiesCriticalityAndConfiguredTimeout(t *testing.T) {
	metrics, err := platformmetrics.New(platformmetrics.Config{
		Role: "api", Environment: "test", Version: "test",
	})
	if err != nil {
		t.Fatalf("New metrics: %v", err)
	}

	checks := []observedChecker{
		{
			next:    commonhealth.CheckerFunc(func(context.Context) error { return errors.New("optional") }),
			metrics: metrics, role: "api", probe: "ready", check: "optional-dependency",
			critical: false, timeout: time.Second,
		},
		{
			next:    commonhealth.CheckerFunc(func(context.Context) error { return errors.New("critical") }),
			metrics: metrics, role: "api", probe: "ready", check: "critical-dependency",
			critical: true, timeout: time.Second,
		},
		{
			next:    commonhealth.CheckerFunc(func(context.Context) error { return context.DeadlineExceeded }),
			metrics: metrics, role: "api", probe: "ready", check: "timed-dependency",
			critical: false, timeout: 10 * time.Second,
		},
	}
	for _, checker := range checks {
		_ = checker.Check(context.Background())
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, sample := range []string{
		`check="optional-dependency",outcome="degraded"`,
		`check="critical-dependency",outcome="unhealthy"`,
		`check="timed-dependency",outcome="timeout"`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}
