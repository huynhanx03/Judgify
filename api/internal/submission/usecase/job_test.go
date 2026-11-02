package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/huynhanx03/judgify/internal/ports"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestJobServiceClaimsOnlyMeasuredFreeLocalCapacity(t *testing.T) {
	repository := &jobServiceRepository{
		leases: []ports.JobLease{
			jobServiceLease(1),
			jobServiceLease(2),
		},
	}
	sink := &jobServiceSink{remaining: 2}
	service := newTestJobService(t, repository, sink, CapacitySnapshot{
		FreeSandboxes: 7, DiskAvailableBytes: 1 << 30,
	})
	claimed, err := service.ClaimAndSchedule(context.Background())
	if err != nil {
		t.Fatalf("ClaimAndSchedule() error = %v", err)
	}
	if claimed != 2 || len(sink.accepted) != 2 {
		t.Fatalf("claimed = %d accepted = %d, want 2", claimed, len(sink.accepted))
	}
	request := repository.claimRequests[0]
	if request.Capacity != 2 {
		t.Fatalf("claim capacity = %d, want min(free=7, remaining=2)", request.Capacity)
	}
	if request.GlobalRunningLimit != 32 || request.PerUserRunningLimit != 2 {
		t.Fatalf(
			"running limits = global:%d user:%d, want 32/2",
			request.GlobalRunningLimit,
			request.PerUserRunningLimit,
		)
	}
}

func TestJobServiceNeverClaimsPastConfiguredLocalDepth(t *testing.T) {
	repository := &jobServiceRepository{leases: []ports.JobLease{jobServiceLease(1)}}
	sink := &jobServiceSink{remaining: 32, depth: 7}
	service := newTestJobService(t, repository, sink, CapacitySnapshot{
		FreeSandboxes: 32, DiskAvailableBytes: 1 << 30,
	})
	claimed, err := service.ClaimAndSchedule(context.Background())
	if err != nil || claimed != 1 {
		t.Fatalf("ClaimAndSchedule() = %d, %v, want one remaining local slot", claimed, err)
	}
	if got := repository.claimRequests[0].Capacity; got != 1 {
		t.Fatalf("claim capacity = %d, want maximum depth minus current depth", got)
	}
}

func TestJobServiceDoesNotTouchPostgreSQLWhenLocalAdmissionIsUnsafe(t *testing.T) {
	tests := []struct {
		name     string
		sink     jobServiceSink
		capacity CapacitySnapshot
	}{
		{
			name: "disk watermark",
			sink: jobServiceSink{remaining: 4},
			capacity: CapacitySnapshot{
				FreeSandboxes: 4, DiskAvailableBytes: 1023,
			},
		},
		{
			name: "bounded depth",
			sink: jobServiceSink{remaining: 4, depth: 8},
			capacity: CapacitySnapshot{
				FreeSandboxes: 4, DiskAvailableBytes: 1 << 30,
			},
		},
		{
			name: "oldest local lease",
			sink: jobServiceSink{remaining: 4, oldestAge: 31 * time.Second},
			capacity: CapacitySnapshot{
				FreeSandboxes: 4, DiskAvailableBytes: 1 << 30,
			},
		},
		{
			name: "no sandbox",
			sink: jobServiceSink{remaining: 4},
			capacity: CapacitySnapshot{
				FreeSandboxes: 0, DiskAvailableBytes: 1 << 30,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &jobServiceRepository{}
			sink := test.sink
			service := newTestJobService(t, repository, &sink, test.capacity)
			claimed, err := service.ClaimAndSchedule(context.Background())
			if claimed != 0 || !errors.Is(err, ErrJobAdmissionClosed) {
				t.Fatalf("ClaimAndSchedule() = %d, %v, want 0/admission closed", claimed, err)
			}
			if len(repository.claimRequests) != 0 {
				t.Fatal("repository claim ran despite unsafe local admission")
			}
		})
	}
}

func TestJobServiceRejectsConcurrentClaimLoopInsteadOfBlocking(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	repository := &jobServiceRepository{
		claim: func(context.Context, ports.JobClaimRequest) ([]ports.JobLease, error) {
			close(entered)
			<-release
			return nil, nil
		},
	}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	first := make(chan error, 1)
	go func() {
		_, err := service.ClaimAndSchedule(context.Background())
		first <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first claim did not enter repository")
	}
	if _, err := service.ClaimAndSchedule(context.Background()); !errors.Is(err, ErrJobAdmissionClosed) {
		t.Fatalf("concurrent ClaimAndSchedule() error = %v, want admission closed", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first ClaimAndSchedule() error = %v", err)
	}
}

func TestJobServiceWipesClaimedTokensWhenLocalHandoffFails(t *testing.T) {
	lease := jobServiceLease(1)
	rawToken := lease.LeaseToken
	repository := &jobServiceRepository{leases: []ports.JobLease{lease}}
	sinkErr := errors.New("scheduler overloaded")
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1, enqueueErr: sinkErr},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	observer := &leaseMetricRecorder{counts: make(map[LeaseMetricOutcome]int)}
	service.config.Observer = observer
	claimed, err := service.ClaimAndSchedule(context.Background())
	if claimed != 0 || !errors.Is(err, sinkErr) {
		t.Fatalf("ClaimAndSchedule() = %d, %v, want handoff error", claimed, err)
	}
	for index, value := range rawToken {
		if value != 0 {
			t.Fatalf("raw token byte %d survived failed handoff", index)
		}
	}
	if observer.counts[LeaseMetricSuccess] != 0 ||
		observer.counts[LeaseMetricRejected] != 1 {
		t.Fatalf("failed handoff metric counts = %v, want one rejection", observer.counts)
	}
}

func TestJobServiceBuildsWorkerAndLeaseHeartbeatsFromOneBootIdentity(t *testing.T) {
	repository := &jobServiceRepository{}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if err := service.HeartbeatWorkerBoot(context.Background()); err != nil {
		t.Fatalf("HeartbeatWorkerBoot() error = %v", err)
	}
	lease := jobServiceLease(1)
	acceptedUntil, err := service.Heartbeat(context.Background(), lease)
	if err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if repository.workerHeartbeat.WorkerID != service.config.WorkerID ||
		repository.workerHeartbeat.WorkerBootID != service.config.WorkerBootID {
		t.Fatal("worker heartbeat did not preserve configured boot identity")
	}
	heartbeat := repository.leaseHeartbeat
	if heartbeat.JobID != lease.Job.ID || heartbeat.AttemptID != lease.Attempt.ID ||
		heartbeat.WorkerID != service.config.WorkerID ||
		heartbeat.WorkerBootID != service.config.WorkerBootID ||
		heartbeat.LeaseOwner != service.config.LeaseOwner ||
		!heartbeat.ExtendTo.Equal(now.Add(service.config.LeaseDuration)) {
		t.Fatalf("lease heartbeat = %#v, want exact configured fence", heartbeat)
	}
	if !acceptedUntil.Equal(heartbeat.ExtendTo) {
		t.Fatalf(
			"accepted lease deadline = %s, want repository deadline %s",
			acceptedUntil,
			heartbeat.ExtendTo,
		)
	}
}

func TestJobServicePassesBoundedReaperPolicy(t *testing.T) {
	repository := &jobServiceRepository{
		reapResult: ports.ReapResult{Promoted: 2, Retried: 1, Dead: 1},
	}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	result, err := service.ReapExpired(context.Background())
	if err != nil {
		t.Fatalf("ReapExpired() error = %v", err)
	}
	if result != repository.reapResult {
		t.Fatalf("ReapExpired() = %#v, want %#v", result, repository.reapResult)
	}
	if repository.reapRequest.Limit != 64 ||
		repository.reapRequest.RetryInitial != time.Second ||
		repository.reapRequest.RetryMaximum != time.Minute {
		t.Fatalf("reap request = %#v, want configured bounds", repository.reapRequest)
	}
}

func TestJobServiceEmitsOnlyClosedAggregateLeaseOutcomes(t *testing.T) {
	repository := &jobServiceRepository{
		leases:     []ports.JobLease{jobServiceLease(1), jobServiceLease(2)},
		reapResult: ports.ReapResult{Promoted: 1, Retried: 2, Dead: 1, Cancelled: 1},
	}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 2},
		CapacitySnapshot{FreeSandboxes: 2, DiskAvailableBytes: 1 << 30},
	)
	observer := &leaseMetricRecorder{counts: make(map[LeaseMetricOutcome]int)}
	service.config.Observer = observer
	if _, err := service.ClaimAndSchedule(context.Background()); err != nil {
		t.Fatalf("ClaimAndSchedule: %v", err)
	}
	if _, err := service.ReapExpired(context.Background()); err != nil {
		t.Fatalf("ReapExpired: %v", err)
	}
	if observer.counts[LeaseMetricSuccess] != 2 ||
		observer.counts[LeaseMetricRetry] != 3 ||
		observer.counts[LeaseMetricDead] != 1 ||
		observer.counts[LeaseMetricCancelled] != 1 {
		t.Fatalf("lease metric counts = %v", observer.counts)
	}
}

func TestJobServiceAcknowledgesOnlyAValidLeaseCapability(t *testing.T) {
	lease := jobServiceLease(1)
	repository := &jobServiceRepository{}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	if err := service.AcknowledgeCancellation(context.Background(), lease); err != nil {
		t.Fatalf("AcknowledgeCancellation() error = %v", err)
	}
	if repository.cancellationLease.Job.ID != lease.Job.ID ||
		repository.cancellationLease.Attempt.ID != lease.Attempt.ID {
		t.Fatalf("acknowledged lease = %#v, want exact capability", repository.cancellationLease)
	}
	lease.LeaseToken = nil
	if err := service.AcknowledgeCancellation(context.Background(), lease); !errors.Is(err, ErrJobServiceInvalid) {
		t.Fatalf("AcknowledgeCancellation() error = %v, want validation failure", err)
	}
}

func TestJobServiceTransitionsWithExactAggregateVersion(t *testing.T) {
	lease := jobServiceLease(1)
	repository := &jobServiceRepository{
		transitionResult: entity.SubmissionState{
			ID: lease.Job.SubmissionID, LatestGeneration: lease.Job.Generation,
			LatestPhase: entity.PhaseCompiling, AggregateVersion: 3,
		},
	}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	state, err := service.TransitionPhase(
		context.Background(),
		lease,
		lease.SubmissionVersion,
		entity.PhaseCompiling,
	)
	if err != nil {
		t.Fatalf("TransitionPhase() error = %v", err)
	}
	if repository.transitionVersion != lease.SubmissionVersion ||
		repository.transitionPhase != entity.PhaseCompiling ||
		state.AggregateVersion != 3 {
		t.Fatalf(
			"transition = version:%d phase:%s state:%#v",
			repository.transitionVersion,
			repository.transitionPhase,
			state,
		)
	}
}

func TestJobServiceOwnsFullJitterRetryDelay(t *testing.T) {
	lease := jobServiceLease(1)
	repository := &jobServiceRepository{}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	service.random = bytes.NewReader(make([]byte, 64))
	incident := ports.InfrastructureIncident{
		Classification: "infrastructure",
		Code:           "sandbox_start_failed",
		Summary:        "sandbox supervisor did not start",
	}
	if err := service.RecordRetry(context.Background(), lease, incident); err != nil {
		t.Fatalf("RecordRetry() error = %v", err)
	}
	failure := repository.retryFailure
	if failure.Classification != incident.Classification ||
		failure.Code != incident.Code ||
		failure.Summary != incident.Summary ||
		failure.RetryAfter != time.Nanosecond {
		t.Fatalf("failure = %#v, want service-owned minimum full jitter", failure)
	}
}

func TestJobServiceRejectsUnsafeIncidentBeforeRepository(t *testing.T) {
	repository := &jobServiceRepository{}
	service := newTestJobService(
		t,
		repository,
		&jobServiceSink{remaining: 1},
		CapacitySnapshot{FreeSandboxes: 1, DiskAvailableBytes: 1 << 30},
	)
	err := service.RecordRetry(
		context.Background(),
		jobServiceLease(1),
		ports.InfrastructureIncident{
			Classification: "infrastructure",
			Code:           "forged\ncode",
			Summary:        "bounded summary",
		},
	)
	if !errors.Is(err, ErrJobServiceInvalid) || repository.retryCalls != 0 {
		t.Fatalf("RecordRetry() = %v calls=%d, want validation before repository", err, repository.retryCalls)
	}
}

type jobServiceRepository struct {
	mu                sync.Mutex
	leases            []ports.JobLease
	claim             func(context.Context, ports.JobClaimRequest) ([]ports.JobLease, error)
	claimRequests     []ports.JobClaimRequest
	workerHeartbeat   ports.WorkerBootHeartbeat
	leaseHeartbeat    ports.LeaseHeartbeat
	reapRequest       ports.ReapRequest
	reapResult        ports.ReapResult
	cancellationLease ports.JobLease
	cancellationErr   error
	transitionVersion int64
	transitionPhase   entity.Phase
	transitionResult  entity.SubmissionState
	transitionErr     error
	retryFailure      ports.InfrastructureFailure
	retryCalls        int
	retryErr          error
}

type leaseMetricRecorder struct {
	counts map[LeaseMetricOutcome]int
}

func (recorder *leaseMetricRecorder) RecordLeaseEvent(
	_ context.Context,
	outcome LeaseMetricOutcome,
	count int,
) {
	recorder.counts[outcome] += count
}

func (repository *jobServiceRepository) Claim(
	ctx context.Context,
	request ports.JobClaimRequest,
) ([]ports.JobLease, error) {
	repository.mu.Lock()
	repository.claimRequests = append(repository.claimRequests, request)
	claim := repository.claim
	leases := repository.leases
	repository.mu.Unlock()
	if claim != nil {
		return claim(ctx, request)
	}
	return leases, nil
}

func (repository *jobServiceRepository) HeartbeatWorkerBoot(
	_ context.Context,
	heartbeat ports.WorkerBootHeartbeat,
) error {
	repository.workerHeartbeat = heartbeat
	return nil
}

func (repository *jobServiceRepository) Heartbeat(
	_ context.Context,
	heartbeat ports.LeaseHeartbeat,
) (time.Time, error) {
	repository.leaseHeartbeat = heartbeat
	return heartbeat.ExtendTo, nil
}

func (repository *jobServiceRepository) ReapExpired(
	_ context.Context,
	request ports.ReapRequest,
) (ports.ReapResult, error) {
	repository.reapRequest = request
	return repository.reapResult, nil
}

func (repository *jobServiceRepository) AcknowledgeCancellation(
	_ context.Context,
	lease ports.JobLease,
) error {
	repository.cancellationLease = lease
	return repository.cancellationErr
}

func (repository *jobServiceRepository) TransitionPhase(
	_ context.Context,
	_ ports.JobLease,
	expectedSubmissionVersion int64,
	next entity.Phase,
) (entity.SubmissionState, error) {
	repository.transitionVersion = expectedSubmissionVersion
	repository.transitionPhase = next
	return repository.transitionResult, repository.transitionErr
}

func (repository *jobServiceRepository) RecordRetry(
	_ context.Context,
	_ ports.JobLease,
	failure ports.InfrastructureFailure,
) error {
	repository.retryCalls++
	repository.retryFailure = failure
	return repository.retryErr
}

type jobServiceSink struct {
	remaining  int
	depth      int
	oldestAge  time.Duration
	enqueueErr error
	accepted   []ports.JobLease
}

func (sink *jobServiceSink) TryEnqueueBatch(leases []ports.JobLease) error {
	if sink.enqueueErr != nil {
		return sink.enqueueErr
	}
	sink.accepted = append(sink.accepted, leases...)
	return nil
}

func (sink *jobServiceSink) Remaining() int { return sink.remaining }
func (sink *jobServiceSink) Depth() int     { return sink.depth }
func (sink *jobServiceSink) OldestAge(time.Time) time.Duration {
	return sink.oldestAge
}

type fixedCapacityProbe struct {
	snapshot CapacitySnapshot
}

func (probe fixedCapacityProbe) Snapshot(context.Context) (CapacitySnapshot, error) {
	return probe.snapshot, nil
}

func newTestJobService(
	t *testing.T,
	repository DurableJobRepository,
	sink LeaseSink,
	snapshot CapacitySnapshot,
) *JobService {
	t.Helper()
	service, err := NewJobService(
		repository,
		sink,
		fixedCapacityProbe{snapshot: snapshot},
		JobServiceConfig{
			WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
			LeaseOwner: "worker-a", LeaseDuration: 30 * time.Second,
			HeartbeatFreshness: 15 * time.Second,
			GlobalRunningLimit: 32, PerUserRunningLimit: 2,
			MinimumDiskBytes: 1024, MaximumLocalDepth: 8,
			MaximumOldestAge: 30 * time.Second,
			ReapLimit:        64, RetryInitial: time.Second, RetryMaximum: time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("NewJobService() error = %v", err)
	}
	return service
}

func jobServiceLease(sequence int) ports.JobLease {
	token := make([]byte, sha256.Size)
	for index := range token {
		token[index] = byte(sequence + index + 1)
	}
	hash := sha256.Sum256(token)
	jobID := testutil.ID(sequence*10 + 1)
	submissionID := testutil.ID(sequence*10 + 2)
	specID := testutil.ID(sequence*10 + 3)
	attemptID := testutil.ID(sequence*10 + 4)
	now := time.Date(2026, time.July, 28, 11, 0, 0, 0, time.UTC)
	return ports.JobLease{
		Job: entity.JudgeJob{
			ID: jobID, SubmissionID: submissionID, Generation: 1,
			JudgeSpecID: specID, Lane: "regular", State: entity.JobLeased,
			Attempts: 1, MaxAttempts: 5, OriginCID: testutil.ID(sequence*10 + 5).String(),
		},
		Attempt: entity.JudgeAttempt{
			ID: attemptID, JobID: jobID, SubmissionID: submissionID,
			Generation: 1, JudgeSpecID: specID, AttemptNumber: 1,
			LeaseTokenHash: entity.LeaseTokenHash(hash),
			WorkerID:       testutil.ID(1), WorkerBootID: testutil.ID(2),
			State: entity.AttemptCreated, StartedAt: now,
		},
		JudgeSpec: ports.JudgeSpecClaim{
			ID: specID, SubmissionID: submissionID, Generation: 1,
			UserID: testutil.ID(sequence*10 + 6),
		},
		LeaseOwner: "worker-a", LeaseToken: token,
		LeaseUntil: now.Add(time.Minute), SubmissionVersion: 2,
		CreatedAt: now,
	}
}
