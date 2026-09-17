package judge

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestHostCapacityProbeReportsOnlyIdleExecutorsAndTrustedDiskBytes(
	t *testing.T,
) {
	scheduler, err := NewFairScheduler(FairSchedulerConfig{
		Capacity: 8, Concurrency: 3,
	})
	if err != nil {
		t.Fatalf("NewFairScheduler() error = %v", err)
	}
	scheduler.mu.Lock()
	scheduler.owned[schedulerLeaseKey{jobID: uuid.New(), attemptID: uuid.New()}] = struct{}{}
	scheduler.owned[schedulerLeaseKey{jobID: uuid.New(), attemptID: uuid.New()}] = struct{}{}
	scheduler.depth = 1
	scheduler.mu.Unlock()

	probe, err := NewHostCapacityProbe(scheduler, "/var/lib/judgify")
	if err != nil {
		t.Fatalf("NewHostCapacityProbe() error = %v", err)
	}
	probe.diskAvailable = func(path string) (int64, error) {
		if path != "/var/lib/judgify" {
			t.Fatalf("disk path = %q", path)
		}
		return 17 << 30, nil
	}

	snapshot, err := probe.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.FreeSandboxes != 2 || snapshot.DiskAvailableBytes != 17<<30 {
		t.Fatalf("Snapshot() = %#v", snapshot)
	}
}

func TestHostCapacityProbeFailsClosed(t *testing.T) {
	scheduler, err := NewFairScheduler(FairSchedulerConfig{
		Capacity: 2, Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("NewFairScheduler() error = %v", err)
	}
	if _, err := NewHostCapacityProbe(scheduler, "."); !errors.Is(err, ErrCapacityProbeInvalid) {
		t.Fatalf("relative path error = %v, want %v", err, ErrCapacityProbeInvalid)
	}
	probe, err := NewHostCapacityProbe(scheduler, "/var/lib/judgify")
	if err != nil {
		t.Fatalf("NewHostCapacityProbe() error = %v", err)
	}
	probe.diskAvailable = func(string) (int64, error) {
		return 0, errors.New("statfs unavailable")
	}
	if _, err := probe.Snapshot(context.Background()); !errors.Is(err, ErrCapacityProbeInvalid) {
		t.Fatalf("disk error = %v, want %v", err, ErrCapacityProbeInvalid)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := probe.Snapshot(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled error = %v, want context.Canceled", err)
	}
}
