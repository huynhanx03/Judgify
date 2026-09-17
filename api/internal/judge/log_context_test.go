package judge

import (
	"context"
	"testing"

	"github.com/google/uuid"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
)

func TestLeaseLogContextCarriesDurableIdentityWithoutCapability(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	ctx := commonlogger.WithContext(context.Background(), zap.New(core))
	lease := submissionports.JobLease{
		Job: submissionentity.JudgeJob{
			ID:           uuid.MustParse("00000000-0000-7000-8000-000000000001"),
			SubmissionID: uuid.MustParse("00000000-0000-7000-8000-000000000002"),
			JudgeSpecID:  uuid.MustParse("00000000-0000-7000-8000-000000000003"),
			Generation:   7,
			OriginCID:    "job-origin-cid",
		},
		Attempt: submissionentity.JudgeAttempt{
			ID:            uuid.MustParse("00000000-0000-7000-8000-000000000004"),
			AttemptNumber: 2,
		},
		LeaseOwner: "worker-1",
		LeaseToken: []byte("raw-lease-capability"),
	}

	ctx = leaseLogContext(ctx, lease)
	commonlogger.FromContext(ctx).Info("lease operation")

	entries := logs.FilterMessage("lease operation").All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	for key, want := range map[string]any{
		"correlation_id": "job-origin-cid",
		"job_id":         "00000000-0000-7000-8000-000000000001",
		"submission_id":  "00000000-0000-7000-8000-000000000002",
		"judge_spec_id":  "00000000-0000-7000-8000-000000000003",
		"attempt_id":     "00000000-0000-7000-8000-000000000004",
		"generation":     "7",
		"attempt":        "2",
	} {
		if fields[key] != want {
			t.Fatalf("%s = %v, want %v", key, fields[key], want)
		}
	}
	for _, forbidden := range []string{"lease_token", "lease_owner"} {
		if _, exists := fields[forbidden]; exists {
			t.Fatalf("lease capability field %q was logged", forbidden)
		}
	}
}
