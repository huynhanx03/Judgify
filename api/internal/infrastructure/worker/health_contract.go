package worker

import (
	"context"
	"errors"
	"time"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

	processhealth "github.com/huynhanx03/judgify/internal/platform/processhealth"
)

var ErrWorkerReadiness = errors.New("worker readiness: unavailable")

// ReadinessDependencies make every production worker invariant explicit.
// The production runtime must provide checkers bound to its current boot:
// claims state and capability evidence cannot be reused from a previous boot.
type ReadinessDependencies struct {
	Initialized     commonhealth.Checker
	Database        commonhealth.Checker
	CurrentBoot     commonhealth.Checker
	LeaseHeartbeat  commonhealth.Checker
	ClaimsEnabled   commonhealth.Checker
	ExactCapability commonhealth.Checker
	SandboxDaemon   commonhealth.Checker
	CgroupV2        commonhealth.Checker
	Disk            commonhealth.Checker
	RewardDelivery  commonhealth.Checker
}

// WorkerHealthDependencies returns the fixed worker readiness contract for
// processhealth.New. It contains no HTTP or Docker implementation.
func WorkerHealthDependencies(
	dependencies ReadinessDependencies,
) ([]processhealth.Dependency, error) {
	if nilInterface(dependencies.Initialized) || nilInterface(dependencies.Database) ||
		nilInterface(dependencies.CurrentBoot) ||
		nilInterface(dependencies.LeaseHeartbeat) ||
		nilInterface(dependencies.ClaimsEnabled) ||
		nilInterface(dependencies.ExactCapability) ||
		nilInterface(dependencies.SandboxDaemon) ||
		nilInterface(dependencies.CgroupV2) ||
		nilInterface(dependencies.Disk) ||
		nilInterface(dependencies.RewardDelivery) {
		return nil, ErrWorkerReadiness
	}
	checks := []struct {
		name string
		code string
		next commonhealth.Checker
	}{
		{name: "worker-initialized", code: "worker_not_initialized", next: dependencies.Initialized},
		{name: "database", code: "database_unavailable", next: dependencies.Database},
		{name: "current-worker-boot", code: "worker_boot_stale", next: dependencies.CurrentBoot},
		{name: "database-leases", code: "worker_lease_stale", next: dependencies.LeaseHeartbeat},
		{name: "claims-enabled", code: "worker_claims_disabled", next: dependencies.ClaimsEnabled},
		{name: "exact-capability", code: "worker_capability_unavailable", next: dependencies.ExactCapability},
		{name: "sandbox-daemon", code: "sandbox_daemon_unavailable", next: dependencies.SandboxDaemon},
		{name: "cgroup-v2", code: "cgroup_v2_unavailable", next: dependencies.CgroupV2},
		{name: "disk-capacity", code: "worker_disk_pressure", next: dependencies.Disk},
		{name: "solve-reward-delivery", code: "solve_reward_delivery_unavailable", next: dependencies.RewardDelivery},
	}
	result := make([]processhealth.Dependency, 0, len(checks))
	for _, check := range checks {
		result = append(result, processhealth.Dependency{
			Name: check.name, Checker: check.next,
			Probes: commonhealth.ProbeReadiness, Critical: true,
			Timeout: time.Second, ErrorCode: check.code,
		})
	}
	return result, nil
}

// AllHealthy composes several implementation checkers into one fail-closed
// checker when a runtime probe naturally verifies several invariants at once.
func AllHealthy(checkers ...commonhealth.Checker) commonhealth.Checker {
	cloned := append([]commonhealth.Checker(nil), checkers...)
	return commonhealth.CheckerFunc(func(ctx context.Context) error {
		if ctx == nil || len(cloned) == 0 {
			return ErrWorkerReadiness
		}
		for _, checker := range cloned {
			if nilInterface(checker) {
				return ErrWorkerReadiness
			}
			if err := checker.Check(ctx); err != nil {
				return errors.Join(ErrWorkerReadiness, err)
			}
		}
		return nil
	})
}
