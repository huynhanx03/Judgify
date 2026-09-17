package supervisor

import (
	"testing"
	"time"
)

func TestSupervisorMetricsContainOnlyAggregateAccounting(t *testing.T) {
	owned := &ownedBoundary{id: "secret-container-id", currentMemoryBytes: 4096}
	supervisor := &SandboxSupervisor{active: map[string]*ownedBoundary{owned.id: owned}}
	supervisor.recordAccounting(ResourceDelta{
		CPUUsage:           250 * time.Millisecond,
		OOMEvents:          2,
		OOMKills:           1,
		CPUThrottledEvents: 3,
		PIDLimitEvents:     4,
	})

	snapshot := supervisor.Metrics()
	if snapshot.CPUUsage != 250*time.Millisecond || snapshot.MemoryCurrentBytes != 4096 ||
		snapshot.OOMEvents != 2 || snapshot.OOMKills != 1 ||
		snapshot.CPUThrottledEvents != 3 || snapshot.PIDLimitEvents != 4 {
		t.Fatalf("Metrics = %+v", snapshot)
	}
}
