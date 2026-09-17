package worker

import (
	"context"
	"errors"
	"testing"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
)

func TestWorkerHealthRequiresSolveRewardDelivery(t *testing.T) {
	healthy := commonhealth.CheckerFunc(func(context.Context) error { return nil })
	dependencies := ReadinessDependencies{
		Initialized:     healthy,
		Database:        healthy,
		CurrentBoot:     healthy,
		LeaseHeartbeat:  healthy,
		ClaimsEnabled:   healthy,
		ExactCapability: healthy,
		SandboxDaemon:   healthy,
		CgroupV2:        healthy,
		Disk:            healthy,
		RewardDelivery:  healthy,
	}
	checks, err := WorkerHealthDependencies(dependencies)
	if err != nil {
		t.Fatalf("WorkerHealthDependencies() error = %v", err)
	}
	found := false
	for _, check := range checks {
		if check.Name == "solve-reward-delivery" {
			found = check.Critical &&
				check.ErrorCode == "solve_reward_delivery_unavailable"
		}
	}
	if !found {
		t.Fatalf("worker health contract omitted critical reward delivery: %+v", checks)
	}

	dependencies.RewardDelivery = nil
	if _, err := WorkerHealthDependencies(dependencies); !errors.Is(
		err,
		ErrWorkerReadiness,
	) {
		t.Fatalf("missing reward delivery error = %v, want ErrWorkerReadiness", err)
	}
}
