package observability

import (
	"context"
	"fmt"
	"os"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"
)

// RecordHealth exports one safe health status. check must be a fixed registry
// name, never a dependency error.
func (metrics *Metrics) RecordHealth(
	ctx context.Context,
	role string,
	probe string,
	check string,
	outcome string,
) {
	_ = metrics.Inc(ctx, MetricHealthChecks, commonmetrics.Labels{
		"role": role, "probe": probe, "check": check, "outcome": outcome,
	})
}

func residentMemoryBytes() (uint64, error) {
	// runtime.MemStats is portable and always exported separately. Linux RSS
	// is read from the process statm file without adding a host dependency.
	contents, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	var totalPages, residentPages uint64
	if _, err := fmt.Sscanf(string(contents), "%d %d", &totalPages, &residentPages); err != nil {
		return 0, err
	}
	pageSize := uint64(os.Getpagesize())
	if residentPages > ^uint64(0)/pageSize {
		return 0, ErrInvalidMetrics
	}
	return residentPages * pageSize, nil
}
