// Package observability owns Judgify's code-defined metric catalog. It keeps
// product metric names and label values out of go-common while reusing the
// generic bounded registry and Prometheus transport.
package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
	commonent "github.com/huynhanx03/go-common/pkg/database/ent"
)

type Metric string

const (
	MetricBuildInfo                   Metric = "judgify_build_info"
	MetricHTTPRequests                Metric = "judgify_http_requests_total"
	MetricHTTPRequestDuration         Metric = "judgify_http_request_duration_seconds"
	MetricDatabasePoolConnections     Metric = "judgify_database_pool_connections"
	MetricDatabaseQueryDuration       Metric = "judgify_database_query_duration_seconds"
	MetricJudgeJobs                   Metric = "judgify_judge_jobs"
	MetricJudgeOldestReady            Metric = "judgify_judge_oldest_ready_job_seconds"
	MetricJudgeWorkerCapacity         Metric = "judgify_judge_worker_capacity"
	MetricJudgeLeaseEvents            Metric = "judgify_judge_lease_events_total"
	MetricJudgeFinalizations          Metric = "judgify_judge_finalizations_total"
	MetricSolveRewardForgeStorage     Metric = "judgify_solve_reward_forge_storage_bytes"
	MetricSolveRewardForgeSegments    Metric = "judgify_solve_reward_forge_segments"
	MetricSolveRewardForgeConsumerLag Metric = "judgify_solve_reward_forge_consumer_lag"
	MetricSolveRewardForgeDLQRecords  Metric = "judgify_solve_reward_forge_dlq_records"
	MetricOutboxEvents                Metric = "judgify_outbox_events"
	MetricOutboxOldestPending         Metric = "judgify_outbox_oldest_pending_seconds"
	MetricOutboxReplayLag             Metric = "judgify_outbox_replay_lag_seconds"
	MetricWebSocketConnections        Metric = "judgify_websocket_connections"
	MetricWebSocketMessages           Metric = "judgify_websocket_messages_total"
	MetricWebSocketCloses             Metric = "judgify_websocket_closes_total"
	MetricSecurityEmailJobs           Metric = "judgify_security_email_jobs"
	MetricSecurityEmailOldestPending  Metric = "judgify_security_email_oldest_pending_seconds"
	MetricCampaignOperations          Metric = "judgify_notification_campaign_operations"
	MetricOperations                  Metric = "judgify_operations"
	MetricContestProjectionLag        Metric = "judgify_contest_projection_lag_seconds"
	MetricContestRatingLag            Metric = "judgify_contest_rating_lag_seconds"
	MetricAuthorizationRevision       Metric = "judgify_authorization_revision"
	MetricHealthChecks                Metric = "judgify_health_checks_total"
	MetricHostDiskAvailable           Metric = "judgify_host_disk_available_bytes"
	MetricSandboxCgroupEvents         Metric = "judgify_sandbox_cgroup_events_total"
	MetricSandboxMemoryCurrent        Metric = "judgify_sandbox_memory_current_bytes"
	MetricSandboxCPU                  Metric = "judgify_sandbox_cpu_seconds_total"
	MetricProcessUptime               Metric = "judgify_process_uptime_seconds"
	MetricProcessResidentMemory       Metric = "judgify_process_resident_memory_bytes"
	MetricProcessHeap                 Metric = "judgify_process_heap_bytes"
	MetricProcessGoroutines           Metric = "judgify_process_goroutines"
	MetricProcessOpenFDs              Metric = "judgify_process_open_fds"
)

var ErrInvalidMetrics = errors.New("judgify observability: invalid metrics")
var ErrCollectorExists = errors.New("judgify observability: collector already registered")
var ErrCollectorLimit = errors.New("judgify observability: collector limit reached")

var durationBuckets = []float64{
	0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
}

type Config struct {
	Role        string
	Environment string
	Version     string
}

// Metrics is the complete process-local catalog. Callers record through Metric
// constants so misspelled, ad-hoc names cannot silently create a new series.
type Metrics struct {
	registry       *commonmetrics.Registry
	instruments    map[Metric]*commonmetrics.Instrument
	startedAt      time.Time
	collectorMu    sync.RWMutex
	collectors     []registeredCollector
	collectorNames map[string]struct{}
	scrapeMu       sync.Mutex
}

func New(config Config) (*Metrics, error) {
	if !oneOf(config.Role, "api", "judge-worker", "migrate") ||
		!safeBuildLabel(config.Environment) || !safeBuildLabel(config.Version) {
		return nil, ErrInvalidMetrics
	}
	registry, err := commonmetrics.NewRegistry(commonmetrics.Options{
		MaxMetrics: 64,
		MaxSeries:  4096,
	})
	if err != nil {
		return nil, err
	}
	metrics := &Metrics{
		registry: registry, instruments: make(map[Metric]*commonmetrics.Instrument),
		// Retain Go's monotonic clock reading for uptime; wall-clock adjustment
		// must never make a process-age gauge move backwards.
		startedAt:      time.Now(),
		collectorNames: make(map[string]struct{}),
	}
	for _, descriptor := range catalog() {
		instrument, registerErr := registry.Register(descriptor)
		if registerErr != nil {
			return nil, fmt.Errorf("register metric %s: %w", descriptor.Name, registerErr)
		}
		metrics.instruments[Metric(descriptor.Name)] = instrument
	}
	if err := metrics.Set(context.Background(), MetricBuildInfo, 1, commonmetrics.Labels{
		"role": config.Role, "environment": config.Environment, "version": config.Version,
	}); err != nil {
		return nil, err
	}
	return metrics, nil
}

func (metrics *Metrics) Add(
	ctx context.Context,
	metric Metric,
	value float64,
	labels commonmetrics.Labels,
) error {
	instrument, err := metrics.instrument(metric)
	if err != nil {
		return err
	}
	return instrument.Add(ctx, value, labels)
}

func (metrics *Metrics) Inc(
	ctx context.Context,
	metric Metric,
	labels commonmetrics.Labels,
) error {
	return metrics.Add(ctx, metric, 1, labels)
}

func (metrics *Metrics) Set(
	ctx context.Context,
	metric Metric,
	value float64,
	labels commonmetrics.Labels,
) error {
	instrument, err := metrics.instrument(metric)
	if err != nil {
		return err
	}
	return instrument.Set(ctx, value, labels)
}

func (metrics *Metrics) Observe(
	ctx context.Context,
	metric Metric,
	value float64,
	labels commonmetrics.Labels,
) error {
	instrument, err := metrics.instrument(metric)
	if err != nil {
		return err
	}
	return instrument.Observe(ctx, value, labels)
}

// Handler captures cheap runtime gauges and bounded registered collectors
// immediately before each scrape.
func (metrics *Metrics) Handler() http.Handler {
	if metrics == nil {
		return commonmetrics.HTTPHandler(nil)
	}
	exporter := commonmetrics.HTTPHandler(metrics.registry)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Match the exporter contract before doing any runtime or dependency
		// work. Unsupported methods must be a cheap 405 and must not advance
		// delta-collector checkpoints.
		if request.Method == http.MethodGet {
			metrics.captureRuntime(request.Context())
			metrics.collectBeforeScrape(request.Context())
		}
		exporter.ServeHTTP(writer, request)
	})
}

// ObserveDatabase implements the generic Ent statement observer using only
// closed statement and outcome classes. Query text, error text, tables,
// arguments, principals, and correlation identifiers are never metric labels.
func (metrics *Metrics) ObserveDatabase(
	ctx context.Context,
	observation commonent.StatementObservation,
) {
	if metrics == nil || ctx == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.WithoutCancel(ctx)
	}
	queryClass := string(observation.Class)
	if !oneOf(queryClass, "read", "write", "transaction", "other") {
		queryClass = "other"
	}
	outcome := "success"
	switch {
	case errors.Is(observation.Err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(observation.Err, context.Canceled):
		outcome = "cancelled"
	case observation.Err != nil:
		outcome = "error"
	}
	duration := observation.Duration
	if duration < 0 {
		duration = 0
	}
	_ = metrics.Observe(ctx, MetricDatabaseQueryDuration, duration.Seconds(), commonmetrics.Labels{
		"query_class": queryClass,
		"outcome":     outcome,
	})
}

func (metrics *Metrics) instrument(metric Metric) (*commonmetrics.Instrument, error) {
	if metrics == nil {
		return nil, ErrInvalidMetrics
	}
	instrument := metrics.instruments[metric]
	if instrument == nil {
		return nil, fmt.Errorf("%w: unknown metric %q", ErrInvalidMetrics, metric)
	}
	return instrument, nil
}

func (metrics *Metrics) captureRuntime(ctx context.Context) {
	if metrics == nil || ctx == nil || ctx.Err() != nil {
		return
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	_ = metrics.Set(ctx, MetricProcessUptime, time.Since(metrics.startedAt).Seconds(), nil)
	_ = metrics.Set(ctx, MetricProcessHeap, float64(memory.HeapAlloc), nil)
	_ = metrics.Set(ctx, MetricProcessGoroutines, float64(runtime.NumGoroutine()), nil)
	if resident, err := residentMemoryBytes(); err == nil {
		_ = metrics.Set(ctx, MetricProcessResidentMemory, float64(resident), nil)
	}
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		_ = metrics.Set(ctx, MetricProcessOpenFDs, float64(len(entries)), nil)
	}
}

func catalog() []commonmetrics.Descriptor {
	enum := func(name string, values ...string) commonmetrics.LabelSpec {
		return commonmetrics.LabelSpec{Name: name, Values: values}
	}
	token := func(name string, maximum int) commonmetrics.LabelSpec {
		return commonmetrics.LabelSpec{Name: name, MaxValues: maximum}
	}
	gauge := func(metric Metric, help string, maximum int, labels ...commonmetrics.LabelSpec) commonmetrics.Descriptor {
		return commonmetrics.Descriptor{
			Name: string(metric), Help: help, Kind: commonmetrics.GaugeKind,
			Labels: labels, MaxSeries: maximum,
		}
	}
	counter := func(metric Metric, help string, maximum int, labels ...commonmetrics.LabelSpec) commonmetrics.Descriptor {
		return commonmetrics.Descriptor{
			Name: string(metric), Help: help, Kind: commonmetrics.CounterKind,
			Labels: labels, MaxSeries: maximum,
		}
	}
	histogram := func(metric Metric, help string, maximum int, labels ...commonmetrics.LabelSpec) commonmetrics.Descriptor {
		return commonmetrics.Descriptor{
			Name: string(metric), Help: help, Kind: commonmetrics.HistogramKind,
			Labels: labels, Buckets: durationBuckets, MaxSeries: maximum,
		}
	}

	roles := enum("role", "api", "judge-worker", "migrate")
	states := enum("state", "ready", "leased", "completed", "cancelled", "dead")
	outcomes := enum("outcome", "success", "retry", "rejected", "dead", "cancelled", "error")
	return []commonmetrics.Descriptor{
		gauge(MetricBuildInfo, "Immutable process build identity.", 48,
			roles, token("environment", 4), token("version", 4)),
		counter(MetricHTTPRequests, "Completed HTTP requests by route template and status class.", 4096,
			roles,
			enum("method", "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "OTHER"),
			token("route", 256),
			enum("status_class", "1xx", "2xx", "3xx", "4xx", "5xx")),
		histogram(MetricHTTPRequestDuration, "HTTP request latency by route template and status class.", 4096,
			roles,
			enum("method", "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "OTHER"),
			token("route", 256),
			enum("status_class", "1xx", "2xx", "3xx", "4xx", "5xx")),
		gauge(MetricDatabasePoolConnections, "Database pool connections by lifecycle state.", 8,
			enum("state", "open", "in_use", "idle", "wait_count")),
		histogram(MetricDatabaseQueryDuration, "Database query latency by code-owned query class.", 256,
			enum("query_class", "read", "write", "transaction", "other"),
			enum("outcome", "success", "timeout", "cancelled", "error")),
		gauge(MetricJudgeJobs, "Durable judge jobs by state.", 8, states),
		gauge(MetricJudgeOldestReady, "Age of the oldest claimable judge job.", 1),
		gauge(MetricJudgeWorkerCapacity, "Judge worker capacity by kind.", 8,
			enum("kind", "total", "free", "running", "claims_enabled")),
		counter(MetricJudgeLeaseEvents, "Judge lease transitions.", 16, outcomes),
		counter(MetricJudgeFinalizations, "Judge terminal finalization attempts.", 16, outcomes),
		gauge(MetricSolveRewardForgeStorage, "Worker-owned solve-reward Forge storage by fixed queue and bound.", 8,
			enum("queue", "main", "dlq"), enum("kind", "used", "limit")),
		gauge(MetricSolveRewardForgeSegments, "Worker-owned solve-reward Forge segments by fixed queue.", 2,
			enum("queue", "main", "dlq")),
		gauge(MetricSolveRewardForgeConsumerLag, "Durable solve-reward Forge consumer lag.", 1),
		gauge(MetricSolveRewardForgeDLQRecords, "Retained solve-reward Forge dead-letter records.", 1),
		gauge(MetricOutboxEvents, "Durable outbox events by state.", 8,
			enum("state", "pending", "leased", "published", "dead")),
		gauge(MetricOutboxOldestPending, "Age of the oldest deliverable outbox event.", 1),
		gauge(MetricOutboxReplayLag, "Lag of durable event replay and reconciliation.", 1),
		gauge(MetricWebSocketConnections, "Active WebSocket connections.", 1),
		counter(MetricWebSocketMessages, "WebSocket message outcomes.", 32,
			enum("direction", "inbound", "outbound"),
			enum("outcome", "accepted", "delivered", "dropped", "rate_limited", "invalid", "error")),
		counter(MetricWebSocketCloses, "WebSocket closes grouped into code-owned classes.", 16,
			enum("class", "normal", "shutdown", "auth", "policy", "slow_client", "protocol", "internal")),
		gauge(MetricSecurityEmailJobs, "Durable security email deliveries by state.", 8,
			enum("state", "pending", "leased", "sent", "dead", "cancelled")),
		gauge(MetricSecurityEmailOldestPending, "Age of the oldest pending security email.", 1),
		gauge(MetricCampaignOperations, "Notification campaign operations by state.", 16,
			enum("state", "scheduled", "running", "paused", "completed", "cancelled", "failed")),
		gauge(MetricOperations, "Durable administrative operations by bounded kind and state.", 512,
			token("kind", 32),
			enum("state", "scheduled", "running", "paused", "completed", "cancelled", "failed")),
		gauge(MetricContestProjectionLag, "Contest projection lag by projection class.", 16,
			enum("projection", "standing", "content", "notification", "lifecycle")),
		gauge(MetricContestRatingLag, "Age of the oldest completed contest awaiting rating.", 1),
		gauge(MetricAuthorizationRevision, "Authorization desired and applied revisions.", 4,
			enum("kind", "desired", "applied")),
		counter(MetricHealthChecks, "Health check outcomes by role, probe, and code-owned check.", 512,
			roles, enum("probe", "live", "ready"), token("check", 128),
			enum("outcome", "healthy", "degraded", "unhealthy", "timeout")),
		gauge(MetricHostDiskAvailable, "Available bytes on code-owned storage classes.", 8,
			enum("storage", "database", "judge_staging", "backup_staging")),
		counter(MetricSandboxCgroupEvents, "Sandbox cgroup pressure and limit events.", 16,
			enum("event", "oom", "oom_kill", "cpu_throttled", "pids_max")),
		gauge(MetricSandboxMemoryCurrent, "Memory currently charged to active sandbox cgroups.", 1),
		counter(MetricSandboxCPU, "CPU time charged to completed sandbox cgroups.", 1),
		gauge(MetricProcessUptime, "Process uptime.", 1),
		gauge(MetricProcessResidentMemory, "Resident process memory when supported by the host.", 1),
		gauge(MetricProcessHeap, "Go heap bytes currently allocated.", 1),
		gauge(MetricProcessGoroutines, "Current Go goroutine count.", 1),
		gauge(MetricProcessOpenFDs, "Open process file descriptors on supported hosts.", 1),
	}
}

func safeBuildLabel(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '_' || char == '-' || char == '+' {
			continue
		}
		return false
	}
	return true
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
