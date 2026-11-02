package infrastructure

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
)

func TestAPIHealthRegistryAggregatesCriticalReadinessAndJanitorDegradation(t *testing.T) {
	t.Parallel()

	database := &healthStub{}
	registry, liveness, err := NewAPIHealthRegistry(APIHealthDependencies{
		Database:                    database,
		EventDelivery:               &healthStub{},
		Authorization:               &healthStub{},
		RealtimeHub:                 &healthStub{},
		RealtimeAuthorizationTailer: &healthStub{},
		SecurityEmail:               &healthStub{err: errors.New("provider queue unavailable")},
		ReceiptJanitor:              &healthStub{err: errors.New("cleanup unavailable")},
		OperationRunner:             &healthStub{},
		DebugWindow:                 &healthStub{},
	})
	require.NoError(t, err)

	runContext, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- liveness.Run(runContext) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-runDone)
	})
	require.Eventually(t, func() bool {
		return registry.CheckLiveness(context.Background()).Healthy
	}, time.Second, time.Millisecond)

	report := registry.CheckReadiness(context.Background())
	require.True(t, report.Healthy)
	require.Equal(t, []commonhealth.Status{
		{Name: "admission-gate", Healthy: true, Critical: true},
		{Name: "authorization", Healthy: true, Critical: true},
		{Name: "database", Healthy: true, Critical: true},
		{Name: "debug-window", Healthy: true, Critical: true},
		{Name: "event-delivery", Healthy: true, Critical: true},
		{Name: "operation-runner", Healthy: true, Critical: false},
		{Name: "realtime-authorization-tailer", Healthy: true, Critical: true},
		{Name: "realtime-hub", Healthy: true, Critical: true},
		{Name: "receipt-janitor", Healthy: false, Critical: false, ErrorCode: "receipt_janitor_degraded"},
		{Name: "security-email-dispatcher", Healthy: false, Critical: false, ErrorCode: "security_email_dispatcher_degraded"},
	}, withoutDurations(report.Statuses))

	database.err = errors.New("postgres://user:secret@db")
	report = registry.CheckReadiness(context.Background())
	require.False(t, report.Healthy)
	require.Equal(t, "database_unavailable", statusNamed(t, report, "database").ErrorCode)
}

func TestAPIHealthRoutesExposeLiveReadyAndCompatiblePing(t *testing.T) {
	t.Parallel()

	registry, liveness, err := NewAPIHealthRegistry(APIHealthDependencies{
		Database:                    &healthStub{},
		EventDelivery:               &healthStub{},
		Authorization:               &healthStub{},
		RealtimeHub:                 &healthStub{},
		RealtimeAuthorizationTailer: &healthStub{},
		SecurityEmail:               &healthStub{},
		ReceiptJanitor:              &healthStub{},
		OperationRunner:             &healthStub{},
		DebugWindow:                 &healthStub{},
	})
	require.NoError(t, err)
	runContext, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- liveness.Run(runContext) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-runDone)
	})
	require.Eventually(t, func() bool {
		return registry.CheckLiveness(context.Background()).Healthy
	}, time.Second, time.Millisecond)

	engine := gin.New()
	registerHealthRoutes(engine, registry)
	engine.GET("/ping", Ping)
	for _, path := range []string{"/live", "/ready", "/ping"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, path)
	}
}

func TestAPIHealthRegistryRejectsMissingCriticalDependencies(t *testing.T) {
	t.Parallel()

	_, _, err := NewAPIHealthRegistry(APIHealthDependencies{})
	require.ErrorIs(t, err, ErrInvalidAPIHealth)
}

type healthStub struct{ err error }

func (stub *healthStub) Check(context.Context) error { return stub.err }

func withoutDurations(statuses []commonhealth.Status) []commonhealth.Status {
	result := append([]commonhealth.Status(nil), statuses...)
	for index := range result {
		result[index].Duration = 0
	}
	return result
}

func statusNamed(t *testing.T, report commonhealth.Report, name string) commonhealth.Status {
	t.Helper()
	for _, status := range report.Statuses {
		if status.Name == name {
			return status
		}
	}
	t.Fatalf("status %q is missing", name)
	return commonhealth.Status{}
}
