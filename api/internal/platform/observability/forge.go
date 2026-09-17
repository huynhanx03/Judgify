package observability

import (
	"context"
	"errors"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
)

// ForgeRuntimeSnapshot contains only fixed-cardinality worker-owned queue
// aggregates. Topic/group names and durable event identifiers never become
// labels.
type ForgeRuntimeSnapshot struct {
	MainStorageBytes      int64
	MainStorageLimitBytes int64
	MainSegments          int
	ConsumerLag           uint64
	DLQStorageBytes       int64
	DLQStorageLimitBytes  int64
	DLQSegments           int
	DLQRecords            uint64
}

type ForgeRuntimeSource interface {
	ForgeMetrics(context.Context) (ForgeRuntimeSnapshot, error)
}

type forgeRuntimeCollector struct {
	metrics *Metrics
	source  ForgeRuntimeSource
}

func (metrics *Metrics) RegisterForgeRuntimeCollector(
	source ForgeRuntimeSource,
) error {
	if isNilInterface(source) {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector(
		"solve-reward-forge",
		&forgeRuntimeCollector{metrics: metrics, source: source},
	)
}

func (collector *forgeRuntimeCollector) Collect(ctx context.Context) error {
	if collector == nil || collector.metrics == nil ||
		isNilInterface(collector.source) || ctx == nil {
		return ErrInvalidMetrics
	}
	snapshot, err := collector.source.ForgeMetrics(ctx)
	if err != nil {
		return err
	}
	if snapshot.MainStorageBytes < 0 ||
		snapshot.MainStorageLimitBytes <= 0 ||
		snapshot.MainStorageBytes > snapshot.MainStorageLimitBytes ||
		snapshot.MainSegments <= 0 ||
		snapshot.DLQStorageBytes < 0 ||
		snapshot.DLQStorageLimitBytes <= 0 ||
		snapshot.DLQStorageBytes > snapshot.DLQStorageLimitBytes ||
		snapshot.DLQSegments <= 0 {
		return ErrInvalidMetrics
	}
	queues := []struct {
		name     string
		used     int64
		limit    int64
		segments int
	}{
		{
			name:     "main",
			used:     snapshot.MainStorageBytes,
			limit:    snapshot.MainStorageLimitBytes,
			segments: snapshot.MainSegments,
		},
		{
			name:     "dlq",
			used:     snapshot.DLQStorageBytes,
			limit:    snapshot.DLQStorageLimitBytes,
			segments: snapshot.DLQSegments,
		},
	}
	var failures []error
	for _, queue := range queues {
		failures = appendMetricError(failures, collector.metrics.Set(
			ctx,
			MetricSolveRewardForgeStorage,
			float64(queue.used),
			commonmetrics.Labels{"queue": queue.name, "kind": "used"},
		))
		failures = appendMetricError(failures, collector.metrics.Set(
			ctx,
			MetricSolveRewardForgeStorage,
			float64(queue.limit),
			commonmetrics.Labels{"queue": queue.name, "kind": "limit"},
		))
		failures = appendMetricError(failures, collector.metrics.Set(
			ctx,
			MetricSolveRewardForgeSegments,
			float64(queue.segments),
			commonmetrics.Labels{"queue": queue.name},
		))
	}
	failures = appendMetricError(failures, collector.metrics.Set(
		ctx,
		MetricSolveRewardForgeConsumerLag,
		float64(snapshot.ConsumerLag),
		nil,
	))
	failures = appendMetricError(failures, collector.metrics.Set(
		ctx,
		MetricSolveRewardForgeDLQRecords,
		float64(snapshot.DLQRecords),
		nil,
	))
	return errors.Join(failures...)
}
