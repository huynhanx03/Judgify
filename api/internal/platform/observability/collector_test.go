package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonws "github.com/huynhanx03/go-common/pkg/common/websocket"
)

type websocketSnapshotSource struct {
	snapshot commonws.MetricsSnapshot
}

func (source *websocketSnapshotSource) Metrics() commonws.MetricsSnapshot {
	return source.snapshot
}

func TestWebSocketCollectorDoesNotLoseDeltaOnCancelledScrape(t *testing.T) {
	metrics := newTestMetrics(t)
	source := &websocketSnapshotSource{snapshot: commonws.MetricsSnapshot{
		ActiveConnections: 2,
		InboundAccepted:   7,
		Enqueued:          3,
		Coalesced:         2,
		DroppedLatest:     1,
		DroppedEphemeral:  2,
		CriticalClosed:    3,
		Closes:            commonws.CloseMetricsSnapshot{Policy: 4},
	}}
	collector := &webSocketCollector{metrics: metrics, source: source}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := collector.Collect(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Collect(cancelled) error = %v, want context.Canceled", err)
	}
	if collector.previous.InboundAccepted != 0 || collector.previous.Closes.Policy != 0 {
		t.Fatalf("cancelled collection advanced checkpoint: %+v", collector.previous)
	}
	if err := collector.Collect(context.Background()); err != nil {
		t.Fatalf("Collect(success) error = %v", err)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, sample := range []string{
		`judgify_websocket_connections 2`,
		`judgify_websocket_messages_total{direction="inbound",outcome="accepted"} 7`,
		`judgify_websocket_messages_total{direction="outbound",outcome="accepted"} 5`,
		`judgify_websocket_messages_total{direction="outbound",outcome="dropped"} 6`,
		`judgify_websocket_closes_total{class="policy"} 4`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}

type workerSnapshotSource struct {
	snapshot WorkerRuntimeSnapshot
	err      error
}

func (source *workerSnapshotSource) WorkerMetrics(context.Context) (WorkerRuntimeSnapshot, error) {
	return source.snapshot, source.err
}

func TestWorkerCollectorDoesNotLoseDeltaOnCancelledScrape(t *testing.T) {
	metrics := newTestMetrics(t)
	source := &workerSnapshotSource{snapshot: WorkerRuntimeSnapshot{
		TotalCapacity:             4,
		FreeCapacity:              3,
		ClaimsEnabled:             true,
		DiskAvailableBytes:        4096,
		SandboxCPUSeconds:         2.5,
		SandboxMemoryCurrentBytes: 1024,
		SandboxOOMEvents:          3,
	}}
	collector := &workerRuntimeCollector{metrics: metrics, source: source}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := collector.Collect(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Collect(cancelled) error = %v, want context.Canceled", err)
	}
	if collector.previous.SandboxCPUSeconds != 0 || collector.previous.SandboxOOMEvents != 0 {
		t.Fatalf("cancelled collection advanced checkpoint: %+v", collector.previous)
	}
	if err := collector.Collect(context.Background()); err != nil {
		t.Fatalf("Collect(success) error = %v", err)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, sample := range []string{
		`judgify_judge_worker_capacity{kind="running"} 1`,
		`judgify_host_disk_available_bytes{storage="judge_staging"} 4096`,
		`judgify_sandbox_cpu_seconds_total 2.5`,
		`judgify_sandbox_cgroup_events_total{event="oom"} 3`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}

func TestWorkerCollectorRejectsImpossibleSnapshot(t *testing.T) {
	collector := &workerRuntimeCollector{
		metrics: newTestMetrics(t),
		source: &workerSnapshotSource{snapshot: WorkerRuntimeSnapshot{
			TotalCapacity: 1, FreeCapacity: 2,
		}},
	}
	if err := collector.Collect(context.Background()); !errors.Is(err, ErrInvalidMetrics) {
		t.Fatalf("Collect() error = %v, want ErrInvalidMetrics", err)
	}
}

type forgeSnapshotSource struct {
	snapshot ForgeRuntimeSnapshot
	err      error
}

func (source *forgeSnapshotSource) ForgeMetrics(
	context.Context,
) (ForgeRuntimeSnapshot, error) {
	return source.snapshot, source.err
}

func TestForgeCollectorPublishesFixedCardinalityStorageLagAndDLQ(t *testing.T) {
	metrics := newTestMetrics(t)
	collector := &forgeRuntimeCollector{
		metrics: metrics,
		source: &forgeSnapshotSource{snapshot: ForgeRuntimeSnapshot{
			MainStorageBytes:      1024,
			MainStorageLimitBytes: 4096,
			MainSegments:          2,
			ConsumerLag:           3,
			DLQStorageBytes:       256,
			DLQStorageLimitBytes:  4096,
			DLQSegments:           1,
			DLQRecords:            4,
		}},
	}
	if err := collector.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	body := response.Body.String()
	for _, sample := range []string{
		`judgify_solve_reward_forge_storage_bytes{queue="main",kind="used"} 1024`,
		`judgify_solve_reward_forge_segments{queue="main"} 2`,
		`judgify_solve_reward_forge_consumer_lag 3`,
		`judgify_solve_reward_forge_dlq_records 4`,
	} {
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %q in metrics body", sample)
		}
	}
}
