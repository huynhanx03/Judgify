package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinuxCgroupV2ResolverReadsTrustedAccounting(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroups := filepath.Join(root, "cgroup")
	pid := 421
	containerID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	relative := filepath.Join("user.slice", "docker-"+containerID+".scope")
	mustWriteTestFile(t, filepath.Join(proc, "421", "cgroup"), "0::/"+filepath.ToSlash(relative)+"\n")
	mustWriteTestFile(t, filepath.Join(cgroups, relative, "cpu.stat"), "usage_usec 1250\nuser_usec 1000\nsystem_usec 250\nnr_throttled 3\n")
	mustWriteTestFile(t, filepath.Join(cgroups, relative, "memory.current"), "4096\n")
	mustWriteTestFile(t, filepath.Join(cgroups, relative, "memory.peak"), "8192\n")
	mustWriteTestFile(t, filepath.Join(cgroups, relative, "memory.events"), "low 0\nhigh 0\nmax 0\noom 1\noom_kill 1\n")
	mustWriteTestFile(t, filepath.Join(cgroups, relative, "pids.events"), "max 2\n")

	resolver, err := NewLinuxCgroupV2Resolver(proc, cgroups)
	if err != nil {
		t.Fatalf("NewLinuxCgroupV2Resolver() error = %v", err)
	}
	account, err := resolver.Resolve(context.Background(), containerID, pid)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	t.Cleanup(func() { _ = account.Close() })
	snapshot, err := account.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.CPUUsage != 1250*time.Microsecond ||
		snapshot.MemoryCurrentBytes != 4096 ||
		snapshot.MemoryPeakBytes != 8192 ||
		snapshot.OOMCount != 1 ||
		snapshot.OOMKillCount != 1 ||
		snapshot.CPUThrottledCount != 3 ||
		snapshot.PIDLimitCount != 2 {
		t.Fatalf("Snapshot() = %+v", snapshot)
	}
	if account.ID() == "" || account.ID() == filepath.ToSlash(relative) {
		t.Fatalf("account.ID() = %q, want bounded opaque ID", account.ID())
	}
}

func TestLinuxCgroupV2ResolverFailsClosedOnMalformedMembershipAndCounters(t *testing.T) {
	t.Parallel()

	for _, membership := range []string{
		"",
		"1:name=systemd:/sandbox\n",
		"0::/../../escape\n",
		"0::relative\n",
		"0::/one\n0::/two\n",
	} {
		root := t.TempDir()
		proc := filepath.Join(root, "proc")
		cgroups := filepath.Join(root, "cgroup")
		mustWriteTestFile(t, filepath.Join(proc, "9", "cgroup"), membership)
		if err := os.MkdirAll(cgroups, 0o700); err != nil {
			t.Fatalf("mkdir cgroup root: %v", err)
		}
		resolver, err := NewLinuxCgroupV2Resolver(proc, cgroups)
		if err != nil {
			t.Fatalf("NewLinuxCgroupV2Resolver() error = %v", err)
		}
		containerID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		if _, err := resolver.Resolve(context.Background(), containerID, 9); err == nil {
			t.Fatalf("membership %q resolved", membership)
		}
	}
}

func TestLinuxCgroupV2ResolverRejectsPIDFromAnotherContainer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	proc := filepath.Join(root, "proc")
	cgroups := filepath.Join(root, "cgroup")
	actualContainerID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	claimedContainerID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	relative := filepath.Join("user.slice", "docker-"+actualContainerID+".scope")
	mustWriteTestFile(t, filepath.Join(proc, "9", "cgroup"), "0::/"+filepath.ToSlash(relative)+"\n")
	if err := os.MkdirAll(filepath.Join(cgroups, relative), 0o700); err != nil {
		t.Fatalf("mkdir cgroup: %v", err)
	}
	resolver, err := NewLinuxCgroupV2Resolver(proc, cgroups)
	if err != nil {
		t.Fatalf("NewLinuxCgroupV2Resolver() error = %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), claimedContainerID, 9); err == nil {
		t.Fatal("Resolve() trusted a PID whose cgroup belongs to another container")
	}
}

func TestAccountingDeltaRejectsCounterRegression(t *testing.T) {
	t.Parallel()

	baseline := ResourceSnapshot{
		CPUUsage: 2 * time.Second, OOMCount: 2, OOMKillCount: 2,
		CPUThrottledCount: 4, PIDLimitCount: 3,
	}
	current := ResourceSnapshot{
		CPUUsage: time.Second, OOMCount: 1, OOMKillCount: 1,
		CPUThrottledCount: 3, PIDLimitCount: 2,
	}
	if _, err := accountingDelta(baseline, current); err == nil {
		t.Fatal("accountingDelta() accepted regressing trusted counters")
	}
}

func mustWriteTestFile(t *testing.T, filename string, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filename, err)
	}
	if err := os.WriteFile(filename, []byte(value), 0o600); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}
