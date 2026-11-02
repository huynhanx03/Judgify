package judge

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestSchedulerWeightedDeficitGivesEveryNonEmptyLaneItsShare(t *testing.T) {
	scheduler := newTestFairScheduler(t, 300, 1)
	leases := make([]submissionports.JobLease, 0, 300)
	sequence := 1
	for _, lane := range schedulerLanes {
		for range 100 {
			leases = append(leases, validScheduledLease(sequence, lane))
			sequence++
		}
	}
	if err := scheduler.TryEnqueueBatch(leases); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	counts := map[string]int{}
	for range 100 {
		lease, err := scheduler.take(context.Background())
		if err != nil {
			t.Fatalf("take() error = %v", err)
		}
		counts[lease.Job.Lane]++
		wipeRawLeaseToken(&lease)
	}
	if counts["contest"] != 60 || counts["regular"] != 35 ||
		counts["maintenance"] != 5 {
		t.Fatalf("first 100 turns = %#v, want 60/35/5", counts)
	}
	scheduler.Close()
}

func TestSchedulerBorrowsAllCapacityWhenOnlyOneLaneHasWork(t *testing.T) {
	scheduler := newTestFairScheduler(t, 3, 1)
	leases := []submissionports.JobLease{
		validScheduledLease(1, "maintenance"),
		validScheduledLease(2, "maintenance"),
		validScheduledLease(3, "maintenance"),
	}
	if err := scheduler.TryEnqueueBatch(leases); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	for index := range 3 {
		lease, err := scheduler.take(context.Background())
		if err != nil {
			t.Fatalf("take %d: %v", index, err)
		}
		if lease.Job.Lane != "maintenance" {
			t.Fatalf("take %d lane = %q, want maintenance", index, lease.Job.Lane)
		}
		wipeRawLeaseToken(&lease)
	}
	scheduler.Close()
}

func TestSchedulerOverloadRejectsWholeBatchWithoutTakingTokenOwnership(t *testing.T) {
	scheduler := newTestFairScheduler(t, 2, 1)
	leases := []submissionports.JobLease{
		validScheduledLease(1, "contest"),
		validScheduledLease(2, "regular"),
		validScheduledLease(3, "maintenance"),
	}
	rawTokens := make([][]byte, len(leases))
	for index := range leases {
		rawTokens[index] = append([]byte(nil), leases[index].LeaseToken...)
	}
	err := scheduler.TryEnqueueBatch(leases)
	if !errors.Is(err, ErrSchedulerOverloaded) {
		t.Fatalf("TryEnqueueBatch() error = %v, want overload", err)
	}
	if scheduler.Depth() != 0 {
		t.Fatalf("Depth() = %d, want 0 after atomic rejection", scheduler.Depth())
	}
	for index := range leases {
		if string(leases[index].LeaseToken) != string(rawTokens[index]) {
			t.Fatalf("lease %d token mutated on rejected batch", index)
		}
	}
	scheduler.Close()
}

func TestSchedulerTransfersAndWipesRawLeaseTokens(t *testing.T) {
	scheduler := newTestFairScheduler(t, 2, 1)
	leases := []submissionports.JobLease{
		validScheduledLease(1, "contest"),
		validScheduledLease(2, "regular"),
	}
	queuedRawToken := leases[1].LeaseToken
	if err := scheduler.TryEnqueueBatch(leases); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	for index := range leases {
		if leases[index].LeaseToken != nil {
			t.Fatalf("caller retained raw token %d after ownership transfer", index)
		}
	}
	taken, err := scheduler.take(context.Background())
	if err != nil {
		t.Fatalf("take() error = %v", err)
	}
	if len(taken.LeaseToken) != sha256.Size {
		t.Fatalf("taken token length = %d, want %d", len(taken.LeaseToken), sha256.Size)
	}
	wipeRawLeaseToken(&taken)
	scheduler.Close()
	for index, value := range queuedRawToken {
		if value != 0 {
			t.Fatalf("queued token byte %d survived Close()", index)
		}
	}
}

func TestSchedulerOldestAgeMeasuresLocalQueuePressure(t *testing.T) {
	scheduler := newTestFairScheduler(t, 1, 1)
	enqueuedAt := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	scheduler.now = func() time.Time { return enqueuedAt }
	leases := []submissionports.JobLease{validScheduledLease(1, "regular")}
	if err := scheduler.TryEnqueueBatch(leases); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	if age := scheduler.OldestAge(enqueuedAt.Add(7 * time.Second)); age != 7*time.Second {
		t.Fatalf("OldestAge() = %s, want 7s", age)
	}
	scheduler.Close()
}

func TestSchedulerRunNeverExceedsConfiguredConcurrency(t *testing.T) {
	scheduler := newTestFairScheduler(t, 8, 2)
	leases := make([]submissionports.JobLease, 8)
	for index := range leases {
		leases[index] = validScheduledLease(index+1, schedulerLanes[index%len(schedulerLanes)])
	}
	if err := scheduler.TryEnqueueBatch(leases); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, len(leases))
	var active atomic.Int32
	var maximum atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Run(ctx, func(ctx context.Context, _ submissionports.JobLease) {
			current := active.Add(1)
			for {
				observed := maximum.Load()
				if current <= observed || maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			started <- struct{}{}
			<-ctx.Done()
			active.Add(-1)
		})
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("configured workers did not start")
		}
	}
	select {
	case <-started:
		t.Fatal("scheduler started more than configured concurrency")
	case <-time.After(10 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after cancellation")
	}
	if maximum.Load() != 2 {
		t.Fatalf("maximum concurrency = %d, want 2", maximum.Load())
	}
}

func TestSchedulerRejectsDuplicateLeaseIdentity(t *testing.T) {
	scheduler := newTestFairScheduler(t, 2, 1)
	lease := validScheduledLease(1, "contest")
	err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease, lease})
	if !errors.Is(err, ErrSchedulerInvalid) {
		t.Fatalf("TryEnqueueBatch() error = %v, want invalid", err)
	}
	if scheduler.Depth() != 0 {
		t.Fatalf("Depth() = %d, want 0", scheduler.Depth())
	}
	scheduler.Close()
}

func TestSchedulerRetainsIdentityOwnershipWhileHandlerIsRunning(t *testing.T) {
	scheduler := newTestFairScheduler(t, 2, 1)
	lease := validScheduledLease(1, "contest")
	duplicate := lease
	duplicate.LeaseToken = append([]byte(nil), lease.LeaseToken...)
	batch := []submissionports.JobLease{lease}
	if err := scheduler.TryEnqueueBatch(batch); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Run(ctx, func(ctx context.Context, _ submissionports.JobLease) {
			close(started)
			<-ctx.Done()
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{duplicate}); !errors.Is(err, ErrSchedulerInvalid) {
		t.Fatalf("in-flight duplicate error = %v, want invalid", err)
	}
	if len(duplicate.LeaseToken) != sha256.Size {
		t.Fatal("rejected in-flight duplicate lost caller token ownership")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func newTestFairScheduler(t *testing.T, capacity int, concurrency int) *FairScheduler {
	t.Helper()
	scheduler, err := NewFairScheduler(FairSchedulerConfig{
		Capacity: capacity, Concurrency: concurrency,
	})
	if err != nil {
		t.Fatalf("NewFairScheduler() error = %v", err)
	}
	return scheduler
}

func validScheduledLease(sequence int, lane string) submissionports.JobLease {
	token := make([]byte, sha256.Size)
	for index := range token {
		token[index] = byte(sequence + index + 1)
	}
	hash := sha256.Sum256(token)
	jobID := testutil.ID(sequence*10 + 1)
	submissionID := testutil.ID(sequence*10 + 2)
	specID := testutil.ID(sequence*10 + 3)
	attemptID := testutil.ID(sequence*10 + 4)
	workerID := testutil.ID(sequence*10 + 5)
	bootID := testutil.ID(sequence*10 + 6)
	now := time.Date(2026, time.July, 28, 11, 0, 0, 0, time.UTC)
	return submissionports.JobLease{
		Job: entity.JudgeJob{
			ID: jobID, SubmissionID: submissionID, Generation: 1,
			JudgeSpecID: specID, Lane: lane, State: entity.JobLeased,
			Attempts: 1, MaxAttempts: 5, OriginCID: testutil.ID(sequence*10 + 7).String(),
		},
		Attempt: entity.JudgeAttempt{
			ID: attemptID, JobID: jobID, SubmissionID: submissionID,
			Generation: 1, JudgeSpecID: specID, AttemptNumber: 1,
			LeaseTokenHash: entity.LeaseTokenHash(hash),
			WorkerID:       workerID, WorkerBootID: bootID,
			State: entity.AttemptCreated, StartedAt: now,
		},
		JudgeSpec: submissionports.JudgeSpecClaim{
			ID: specID, SubmissionID: submissionID, Generation: 1,
			UserID: testutil.ID(sequence*10 + 8),
		},
		LeaseOwner: "worker-a", LeaseToken: token,
		LeaseUntil: now.Add(time.Minute), SubmissionVersion: 2,
		CreatedAt: now,
	}
}
