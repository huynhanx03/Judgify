package worker

import commonhealth "github.com/huynhanx03/go-common/pkg/common/health"

// NewWorkerReadinessDependencies binds the runtime and durable SQL snapshots
// into the fixed process-health contract. The returned snapshotter may also be
// used for bounded trusted gauges; it contains no identifying label values.
func NewWorkerReadinessDependencies(
	state *RuntimeReadinessState,
	database ReadinessSQL,
	databaseChecker commonhealth.Checker,
	rewardDelivery commonhealth.Checker,
) (
	ReadinessDependencies,
	*SQLPersistentReadinessSnapshotter,
	error,
) {
	if state == nil || nilInterface(database) || nilInterface(databaseChecker) ||
		nilInterface(rewardDelivery) {
		return ReadinessDependencies{}, nil, ErrWorkerReadiness
	}
	runtimeChecks, err := NewRuntimeReadinessCheckers(state)
	if err != nil {
		return ReadinessDependencies{}, nil, err
	}
	persistent, err := NewSQLPersistentReadinessSnapshotter(database, state)
	if err != nil {
		return ReadinessDependencies{}, nil, err
	}
	persistentChecks, err := NewPersistentReadinessCheckers(persistent)
	if err != nil {
		return ReadinessDependencies{}, nil, err
	}
	return ReadinessDependencies{
		Initialized:     runtimeChecks.Initialized,
		Database:        databaseChecker,
		CurrentBoot:     persistentChecks.CurrentBoot,
		LeaseHeartbeat:  persistentChecks.LeaseHeartbeat,
		ClaimsEnabled:   persistentChecks.ClaimsEnabled,
		ExactCapability: persistentChecks.ExactRelease,
		SandboxDaemon:   runtimeChecks.Sandbox,
		CgroupV2:        runtimeChecks.CgroupV2,
		Disk:            runtimeChecks.Disk,
		RewardDelivery:  rewardDelivery,
	}, persistent, nil
}
