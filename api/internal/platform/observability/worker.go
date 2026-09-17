package observability

import (
	"context"
	"errors"
	"math"
	"sync"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
)

// WorkerRuntimeSnapshot is a product-neutral aggregate supplied by the
// judge-worker composition. It contains no worker, boot, sandbox, job,
// submission, event, or correlation identifiers.
type WorkerRuntimeSnapshot struct {
	TotalCapacity             int
	FreeCapacity              int
	ClaimsEnabled             bool
	DiskAvailableBytes        int64
	SandboxCPUSeconds         float64
	SandboxMemoryCurrentBytes int64
	SandboxOOMEvents          uint64
	SandboxOOMKills           uint64
	SandboxCPUThrottledEvents uint64
	SandboxPIDLimitEvents     uint64
}

type WorkerRuntimeSource interface {
	WorkerMetrics(context.Context) (WorkerRuntimeSnapshot, error)
}

type workerRuntimeCollector struct {
	metrics  *Metrics
	source   WorkerRuntimeSource
	mu       sync.Mutex
	previous WorkerRuntimeSnapshot
}

func (metrics *Metrics) RegisterWorkerRuntimeCollector(source WorkerRuntimeSource) error {
	if isNilInterface(source) {
		return ErrInvalidMetrics
	}
	return metrics.RegisterCollector("judge-worker-runtime", &workerRuntimeCollector{
		metrics: metrics,
		source:  source,
	})
}

func (collector *workerRuntimeCollector) Collect(ctx context.Context) error {
	if collector == nil || collector.metrics == nil || collector.source == nil || ctx == nil {
		return ErrInvalidMetrics
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	current, err := collector.source.WorkerMetrics(ctx)
	if err != nil {
		return err
	}
	if current.TotalCapacity < 0 || current.FreeCapacity < 0 ||
		current.FreeCapacity > current.TotalCapacity || current.DiskAvailableBytes < 0 ||
		current.SandboxMemoryCurrentBytes < 0 || current.SandboxCPUSeconds < 0 ||
		math.IsNaN(current.SandboxCPUSeconds) || math.IsInf(current.SandboxCPUSeconds, 0) {
		return ErrInvalidMetrics
	}
	previous := &collector.previous
	running := current.TotalCapacity - current.FreeCapacity
	claimsEnabled := 0.0
	if current.ClaimsEnabled {
		claimsEnabled = 1
	}
	capacity := []struct {
		kind  string
		value float64
	}{
		{kind: "total", value: float64(current.TotalCapacity)},
		{kind: "free", value: float64(current.FreeCapacity)},
		{kind: "running", value: float64(running)},
		{kind: "claims_enabled", value: claimsEnabled},
	}
	var failures []error
	for _, sample := range capacity {
		failures = appendMetricError(failures, collector.metrics.Set(ctx, MetricJudgeWorkerCapacity, sample.value, commonmetrics.Labels{"kind": sample.kind}))
	}
	failures = appendMetricError(failures, collector.metrics.Set(ctx, MetricHostDiskAvailable, float64(current.DiskAvailableBytes), commonmetrics.Labels{"storage": "judge_staging"}))
	failures = appendMetricError(failures, collector.metrics.Set(ctx, MetricSandboxMemoryCurrent, float64(current.SandboxMemoryCurrentBytes), nil))
	cpuDelta := current.SandboxCPUSeconds - previous.SandboxCPUSeconds
	if cpuDelta < 0 {
		cpuDelta = current.SandboxCPUSeconds
	}
	if err := collector.metrics.Add(ctx, MetricSandboxCPU, cpuDelta, nil); err != nil {
		failures = append(failures, err)
	} else {
		previous.SandboxCPUSeconds = current.SandboxCPUSeconds
	}
	events := []struct {
		name     string
		current  uint64
		previous *uint64
	}{
		{name: "oom", current: current.SandboxOOMEvents, previous: &previous.SandboxOOMEvents},
		{name: "oom_kill", current: current.SandboxOOMKills, previous: &previous.SandboxOOMKills},
		{name: "cpu_throttled", current: current.SandboxCPUThrottledEvents, previous: &previous.SandboxCPUThrottledEvents},
		{name: "pids_max", current: current.SandboxPIDLimitEvents, previous: &previous.SandboxPIDLimitEvents},
	}
	for _, sample := range events {
		if err := collector.metrics.Add(
			ctx,
			MetricSandboxCgroupEvents,
			counterDeltaFloat(sample.current, *sample.previous),
			commonmetrics.Labels{"event": sample.name},
		); err != nil {
			failures = append(failures, err)
		} else {
			*sample.previous = sample.current
		}
	}
	return errors.Join(failures...)
}
