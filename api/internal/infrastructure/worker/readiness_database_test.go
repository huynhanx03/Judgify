package worker

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReadinessSnapshotsExportOnlyDerivedBoundedState(t *testing.T) {
	t.Parallel()
	timestamp := reflect.TypeOf(time.Time{})
	for _, snapshot := range []any{
		PersistentReadinessSnapshot{},
		RuntimeReadinessSnapshot{},
		CapacityReadinessSnapshot{},
	} {
		snapshotType := reflect.TypeOf(snapshot)
		for index := 0; index < snapshotType.NumField(); index++ {
			field := snapshotType.Field(index)
			if field.IsExported() && field.Type == timestamp {
				t.Fatalf(
					"%s exports raw timestamp field %s",
					snapshotType.Name(),
					field.Name,
				)
			}
		}
	}
}

func TestWorkerLeaseReadinessSQLFailsClosedOnNullableLeaseProof(t *testing.T) {
	t.Parallel()
	for _, required := range []string{
		"COALESCE(bool_and(COALESCE(",
		"LEFT JOIN judge_attempts AS attempt",
		"attempt.id IS NOT NULL",
		"attempt.worker_id = $1",
		"attempt.worker_boot_id = $2",
		"job.lease_until IS NOT NULL",
		"job.heartbeat_at IS NOT NULL",
		"attempt.heartbeat_at IS NOT NULL",
		"octet_length(job.lease_token_hash) = 32",
		"octet_length(attempt.lease_token_hash) = 32",
		"job.lease_token_hash = attempt.lease_token_hash",
		", false)), true)",
	} {
		if !strings.Contains(workerLeaseReadinessSQL, required) {
			t.Fatalf("lease readiness SQL missing fail-closed guard %q", required)
		}
	}
}
