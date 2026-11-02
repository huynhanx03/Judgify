package judge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	submissionentity "github.com/huynhanx03/judgify/internal/submission/domain"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
	"github.com/huynhanx03/judgify/internal/testutil"
)

func TestDurableWorkerCancelsLocalExecutionAfterThreeTransientLeaseHeartbeatFailures(t *testing.T) {
	heartbeatErr := errors.New("database temporarily unavailable")
	repository := &durableWorkerRepository{
		leaseHeartbeat: func(
			context.Context,
			int,
			submissionports.LeaseHeartbeat,
		) (time.Time, error) {
			return time.Time{}, heartbeatErr
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(1, "regular", service.LeaseDuration())
	lease.LeaseUntil = time.Now().Add(time.Hour)
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStarted := make(chan struct{})
	executionStopped := make(chan error, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			close(executionStarted)
			<-ctx.Done()
			executionStopped <- ctx.Err()
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case <-executionStarted:
	case <-time.After(time.Second):
		t.Fatal("lease execution did not start")
	}
	select {
	case err := <-executionStopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("executor stopped with %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("executor was not cancelled after heartbeat failures")
	}
	if repository.leaseHeartbeatCount() != consecutiveHeartbeatFailureLimit {
		t.Fatalf(
			"lease heartbeat calls = %d, want %d",
			repository.leaseHeartbeatCount(),
			consecutiveHeartbeatFailureLimit,
		)
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerDefinitiveLeaseFenceCancelsWithoutThreeRetries(t *testing.T) {
	repository := &durableWorkerRepository{
		leaseHeartbeat: func(
			context.Context,
			int,
			submissionports.LeaseHeartbeat,
		) (time.Time, error) {
			return time.Time{}, submissionports.ErrLeaseLost
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(2, "contest", service.LeaseDuration())
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStopped := make(chan struct{})
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			<-ctx.Done()
			close(executionStopped)
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case <-executionStopped:
	case <-time.After(time.Second):
		t.Fatal("definitively fenced executor was not cancelled")
	}
	if repository.leaseHeartbeatCount() != 1 {
		t.Fatalf(
			"lease heartbeat calls = %d, want immediate stop after 1",
			repository.leaseHeartbeatCount(),
		)
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerStopsWhenBootHeartbeatFailsThreeConsecutiveTimes(t *testing.T) {
	heartbeatErr := errors.New("database unavailable")
	repository := &durableWorkerRepository{
		workerHeartbeat: func(_ context.Context, call int) error {
			if call == 1 {
				return nil
			}
			return heartbeatErr
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	worker, err := NewDurableWorker(
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			return nil
		}),
		DurableWorkerConfig{
			ClaimInterval: time.Hour, ReapInterval: time.Hour,
			WorkerHeartbeatInterval: time.Millisecond,
			LeaseHeartbeatInterval:  time.Millisecond,
		},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewDurableWorker() error = %v", err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(context.Background()) }()
	select {
	case runErr := <-runDone:
		if !errors.Is(runErr, heartbeatErr) {
			t.Fatalf("Run() error = %v, want heartbeat failure", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after boot heartbeat failure threshold")
	}
	if repository.workerHeartbeatCount() != consecutiveHeartbeatFailureLimit+1 {
		t.Fatalf(
			"worker heartbeat calls = %d, want startup + %d failures",
			repository.workerHeartbeatCount(),
			consecutiveHeartbeatFailureLimit,
		)
	}
}

func TestDurableWorkerRejectsBootHeartbeatCadenceThatCanAgeOut(t *testing.T) {
	t.Parallel()

	repository := &durableWorkerRepository{}
	scheduler, err := NewFairScheduler(FairSchedulerConfig{
		Capacity: 1, Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("NewFairScheduler() error = %v", err)
	}
	service := newDurableWorkerJobService(t, repository, scheduler)
	_, err = NewDurableWorker(
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			return nil
		}),
		DurableWorkerConfig{
			ClaimInterval:           10 * time.Millisecond,
			ReapInterval:            10 * time.Millisecond,
			WorkerHeartbeatInterval: service.HeartbeatFreshness()/2 + time.Nanosecond,
			LeaseHeartbeatInterval:  10 * time.Millisecond,
		},
		zap.NewNop(),
	)
	if !errors.Is(err, ErrDurableWorkerInvalid) {
		t.Fatalf("NewDurableWorker() error = %v, want ErrDurableWorkerInvalid", err)
	}
}

func TestDurableWorkerRejectsLeaseHeartbeatCadenceWithoutHardExpiryHeadroom(t *testing.T) {
	t.Parallel()

	repository := &durableWorkerRepository{}
	scheduler, err := NewFairScheduler(FairSchedulerConfig{
		Capacity: 1, Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("NewFairScheduler() error = %v", err)
	}
	service := newDurableWorkerJobService(t, repository, scheduler)
	_, err = NewDurableWorker(
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			return nil
		}),
		DurableWorkerConfig{
			ClaimInterval:           10 * time.Millisecond,
			ReapInterval:            10 * time.Millisecond,
			WorkerHeartbeatInterval: 10 * time.Millisecond,
			LeaseHeartbeatInterval:  service.LeaseDuration()/4 + time.Nanosecond,
		},
		zap.NewNop(),
	)
	if !errors.Is(err, ErrDurableWorkerInvalid) {
		t.Fatalf("NewDurableWorker() error = %v, want ErrDurableWorkerInvalid", err)
	}
}

func TestDurableWorkerDoesNotAdmitWorkBeforeInitialBootHeartbeatSucceeds(t *testing.T) {
	heartbeatRetryBlocked := make(chan struct{})
	allowHeartbeatSuccess := make(chan struct{})
	repository := &durableWorkerRepository{
		workerHeartbeat: func(_ context.Context, call int) error {
			switch call {
			case 1:
				return errors.New("database temporarily unavailable")
			case 2:
				close(heartbeatRetryBlocked)
				<-allowHeartbeatSuccess
			}
			return nil
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(5, "regular", service.LeaseDuration())
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStarted := make(chan struct{}, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			executionStarted <- struct{}{}
			return nil
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case <-heartbeatRetryBlocked:
	case <-time.After(time.Second):
		t.Fatal("initial heartbeat was not retried")
	}
	select {
	case <-executionStarted:
		t.Fatal("scheduler admitted a lease before boot heartbeat success")
	default:
	}
	if repository.claimCount() != 0 || repository.reapCount() != 0 {
		t.Fatalf(
			"pre-heartbeat admission calls = claim:%d reap:%d, want zero",
			repository.claimCount(),
			repository.reapCount(),
		)
	}
	close(allowHeartbeatSuccess)
	select {
	case <-executionStarted:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not start after boot heartbeat success")
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerInitialHeartbeatFailureClosesAdmission(t *testing.T) {
	heartbeatErr := errors.New("database unavailable")
	repository := &durableWorkerRepository{
		workerHeartbeat: func(context.Context, int) error {
			return heartbeatErr
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(6, "regular", service.LeaseDuration())
	rawToken := lease.LeaseToken
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStarted := make(chan struct{}, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			executionStarted <- struct{}{}
			return nil
		}),
	)
	runErr := worker.Run(context.Background())
	if !errors.Is(runErr, heartbeatErr) {
		t.Fatalf("Run() error = %v, want initial heartbeat failure", runErr)
	}
	select {
	case <-executionStarted:
		t.Fatal("scheduler admitted a lease without a successful boot heartbeat")
	default:
	}
	for index, value := range rawToken {
		if value != 0 {
			t.Fatalf("queued lease token byte %d survived closed admission", index)
		}
	}
}

func TestDurableWorkerHardExpiryUsesLastDatabaseAcceptedLeaseDeadline(t *testing.T) {
	scheduler := newTestFairScheduler(t, 1, 1)
	repository := &durableWorkerRepository{}
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(7, "contest", service.LeaseDuration())
	lease.LeaseUntil = time.Now().Add(80 * time.Millisecond)
	acceptedUntil := lease.LeaseUntil
	repository.leaseHeartbeat = func(
		context.Context,
		int,
		submissionports.LeaseHeartbeat,
	) (time.Time, error) {
		return acceptedUntil, nil
	}
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStopped := make(chan time.Time, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			<-ctx.Done()
			executionStopped <- time.Now()
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case stoppedAt := <-executionStopped:
		if stoppedAt.Before(acceptedUntil) {
			t.Fatalf("execution stopped at %s before accepted expiry %s", stoppedAt, acceptedUntil)
		}
	case <-time.After(time.Second):
		t.Fatal("execution survived the database-accepted hard lease expiry")
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerRearmsHardExpiryOnlyFromSuccessfulHeartbeatResult(t *testing.T) {
	scheduler := newTestFairScheduler(t, 1, 1)
	repository := &durableWorkerRepository{}
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(8, "regular", service.LeaseDuration())
	lease.LeaseUntil = time.Now().Add(60 * time.Millisecond)
	// The accepted expiry is fixed at the first heartbeat instead of at test
	// setup, so scheduler delay cannot make it elapse before execution starts.
	// Later heartbeats repeat that same instant so the hard expiry still arrives.
	const renewedLeaseWindow = 180 * time.Millisecond
	renewedExpiry := make(chan time.Time, 1)
	var heartbeatMu sync.Mutex
	var acceptedUntil time.Time
	repository.leaseHeartbeat = func(
		context.Context,
		int,
		submissionports.LeaseHeartbeat,
	) (time.Time, error) {
		heartbeatMu.Lock()
		defer heartbeatMu.Unlock()
		if acceptedUntil.IsZero() {
			acceptedUntil = time.Now().Add(renewedLeaseWindow)
			renewedExpiry <- acceptedUntil
		}
		return acceptedUntil, nil
	}
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStopped := make(chan time.Time, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			<-ctx.Done()
			executionStopped <- time.Now()
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	var renewedUntil time.Time
	select {
	case renewedUntil = <-renewedExpiry:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never sent a lease heartbeat")
	}
	// The renewed expiry always lands after the lease's original expiry, so a run
	// that survives past it can only have re-armed from the heartbeat result.
	select {
	case stoppedAt := <-executionStopped:
		if stoppedAt.Before(renewedUntil) {
			t.Fatalf("execution stopped at %s before renewed expiry %s", stoppedAt, renewedUntil)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("execution survived the renewed hard lease expiry")
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerTimeBoxesHungLeaseHeartbeat(t *testing.T) {
	heartbeatStarted := make(chan struct{})
	releaseHeartbeat := make(chan struct{})
	repository := &durableWorkerRepository{
		leaseHeartbeat: func(
			context.Context,
			int,
			submissionports.LeaseHeartbeat,
		) (time.Time, error) {
			close(heartbeatStarted)
			<-releaseHeartbeat
			return time.Time{}, errors.New("released hung heartbeat")
		},
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(9, "regular", service.LeaseDuration())
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStopped := make(chan struct{})
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			<-ctx.Done()
			close(executionStopped)
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case <-heartbeatStarted:
	case <-time.After(time.Second):
		t.Fatal("lease heartbeat did not start")
	}
	select {
	case <-executionStopped:
	case <-time.After(time.Until(lease.LeaseUntil) + 100*time.Millisecond):
		t.Fatal("hung heartbeat allowed execution to outlive the lease")
	}
	close(releaseHeartbeat)
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerPreCancelledRunConsumesLifecycleAndClosesScheduler(t *testing.T) {
	repository := &durableWorkerRepository{}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(10, "regular", service.LeaseDuration())
	rawToken := lease.LeaseToken
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			t.Fatal("pre-cancelled worker executed a lease")
			return nil
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	cancelRun()
	if err := worker.Run(runCtx); err != nil {
		t.Fatalf("pre-cancelled Run() error = %v", err)
	}
	if err := worker.Run(context.Background()); !errors.Is(err, ErrDurableWorkerInvalid) {
		t.Fatalf("second Run() error = %v, want one-shot rejection", err)
	}
	for index, value := range rawToken {
		if value != 0 {
			t.Fatalf("queued lease token byte %d survived pre-cancelled Run", index)
		}
	}
	nextLease := validDurableWorkerLease(11, "regular", service.LeaseDuration())
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{nextLease}); !errors.Is(err, ErrSchedulerClosed) {
		t.Fatalf("TryEnqueueBatch() error = %v, want ErrSchedulerClosed", err)
	}
}

func TestDurableWorkerWipesLeaseCapabilityAfterExecutionAndHeartbeatJoin(t *testing.T) {
	repository := &durableWorkerRepository{}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(3, "regular", service.LeaseDuration())
	rawToken := lease.LeaseToken
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executed := make(chan struct{})
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(context.Context, submissionports.JobLease) error {
			close(executed)
			return nil
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("lease execution did not run")
	}
	deadline := time.After(time.Second)
	for scheduler.FreeExecutionSlots() < 1 {
		select {
		case <-deadline:
			t.Fatal("raw lease capability survived completed execution")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	for index, value := range rawToken {
		if value != 0 {
			t.Fatalf("raw lease capability byte %d not wiped after execution", index)
		}
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

func TestDurableWorkerAcknowledgesCancellationAfterStoppingLeaseExecution(t *testing.T) {
	repository := &durableWorkerRepository{
		leaseHeartbeat: func(
			context.Context,
			int,
			submissionports.LeaseHeartbeat,
		) (time.Time, error) {
			return time.Time{}, submissionports.ErrLeaseCancelled
		},
		cancellationAcknowledged: make(chan submissionports.JobLease, 1),
	}
	scheduler := newTestFairScheduler(t, 1, 1)
	service := newDurableWorkerJobService(t, repository, scheduler)
	lease := validDurableWorkerLease(4, "regular", service.LeaseDuration())
	if err := scheduler.TryEnqueueBatch([]submissionports.JobLease{lease}); err != nil {
		t.Fatalf("TryEnqueueBatch() error = %v", err)
	}
	executionStopped := make(chan error, 1)
	worker := newTestDurableWorker(
		t,
		service,
		scheduler,
		durableLeaseExecutorFunc(func(ctx context.Context, _ submissionports.JobLease) error {
			<-ctx.Done()
			executionStopped <- ctx.Err()
			return ctx.Err()
		}),
	)
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- worker.Run(runCtx) }()
	select {
	case executionErr := <-executionStopped:
		if !errors.Is(executionErr, context.Canceled) {
			t.Fatalf("executor error = %v, want context canceled", executionErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled lease did not stop local execution")
	}
	select {
	case acknowledged := <-repository.cancellationAcknowledged:
		if acknowledged.Job.ID != lease.Job.ID || acknowledged.Attempt.ID != lease.Attempt.ID {
			t.Fatalf("acknowledged wrong lease: job=%s attempt=%s", acknowledged.Job.ID, acknowledged.Attempt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled lease was not acknowledged")
	}
	if repository.retryCount() != 0 {
		t.Fatalf("retry calls = %d, want 0 for a deliberate cancellation", repository.retryCount())
	}
	cancelRun()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop")
	}
}

type durableLeaseExecutorFunc func(context.Context, submissionports.JobLease) error

func (executor durableLeaseExecutorFunc) ExecuteLease(
	ctx context.Context,
	lease submissionports.JobLease,
) error {
	return executor(ctx, lease)
}

type durableWorkerRepository struct {
	mu              sync.Mutex
	workerCalls     int
	leaseCalls      int
	claimCalls      int
	reapCalls       int
	retryCalls      int
	workerHeartbeat func(context.Context, int) error
	leaseHeartbeat  func(
		context.Context,
		int,
		submissionports.LeaseHeartbeat,
	) (time.Time, error)
	cancellationAcknowledged chan submissionports.JobLease
}

func (repository *durableWorkerRepository) Claim(
	_ context.Context,
	_ submissionports.JobClaimRequest,
) ([]submissionports.JobLease, error) {
	repository.mu.Lock()
	repository.claimCalls++
	repository.mu.Unlock()
	return nil, nil
}

func (repository *durableWorkerRepository) HeartbeatWorkerBoot(
	ctx context.Context,
	_ submissionports.WorkerBootHeartbeat,
) error {
	repository.mu.Lock()
	repository.workerCalls++
	call := repository.workerCalls
	heartbeat := repository.workerHeartbeat
	repository.mu.Unlock()
	if heartbeat == nil {
		return nil
	}
	return heartbeat(ctx, call)
}

func (repository *durableWorkerRepository) Heartbeat(
	ctx context.Context,
	heartbeatRequest submissionports.LeaseHeartbeat,
) (time.Time, error) {
	repository.mu.Lock()
	repository.leaseCalls++
	call := repository.leaseCalls
	heartbeat := repository.leaseHeartbeat
	repository.mu.Unlock()
	if heartbeat == nil {
		return heartbeatRequest.ExtendTo, nil
	}
	return heartbeat(ctx, call, heartbeatRequest)
}

func (repository *durableWorkerRepository) ReapExpired(
	context.Context,
	submissionports.ReapRequest,
) (submissionports.ReapResult, error) {
	repository.mu.Lock()
	repository.reapCalls++
	repository.mu.Unlock()
	return submissionports.ReapResult{}, nil
}

func (repository *durableWorkerRepository) AcknowledgeCancellation(
	_ context.Context,
	lease submissionports.JobLease,
) error {
	if repository.cancellationAcknowledged != nil {
		repository.cancellationAcknowledged <- lease
	}
	return nil
}

func (*durableWorkerRepository) TransitionPhase(
	_ context.Context,
	lease submissionports.JobLease,
	expectedVersion int64,
	phase submissionentity.Phase,
) (submissionentity.SubmissionState, error) {
	return submissionentity.SubmissionState{
		ID:               lease.Job.SubmissionID,
		LatestGeneration: lease.Job.Generation,
		LatestPhase:      phase,
		AggregateVersion: expectedVersion + 1,
	}, nil
}

func (repository *durableWorkerRepository) RecordRetry(
	context.Context,
	submissionports.JobLease,
	submissionports.InfrastructureFailure,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.retryCalls++
	return nil
}

func (repository *durableWorkerRepository) leaseHeartbeatCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.leaseCalls
}

func (repository *durableWorkerRepository) workerHeartbeatCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.workerCalls
}

func (repository *durableWorkerRepository) claimCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.claimCalls
}

func (repository *durableWorkerRepository) reapCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.reapCalls
}

func (repository *durableWorkerRepository) retryCount() int {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.retryCalls
}

type durableWorkerCapacity struct{}

func (durableWorkerCapacity) Snapshot(
	context.Context,
) (submissionservice.CapacitySnapshot, error) {
	return submissionservice.CapacitySnapshot{
		FreeSandboxes: 1, DiskAvailableBytes: 1 << 30,
	}, nil
}

func newDurableWorkerJobService(
	t *testing.T,
	repository *durableWorkerRepository,
	scheduler *FairScheduler,
) *submissionservice.JobService {
	t.Helper()
	service, err := submissionservice.NewJobService(
		repository,
		scheduler,
		durableWorkerCapacity{},
		submissionservice.JobServiceConfig{
			WorkerID: testutil.ID(1), WorkerBootID: testutil.ID(2),
			LeaseOwner: "worker-a", LeaseDuration: 100 * time.Millisecond,
			HeartbeatFreshness: 50 * time.Millisecond,
			GlobalRunningLimit: 8, PerUserRunningLimit: 1,
			MinimumDiskBytes: 1, MaximumLocalDepth: 1,
			MaximumOldestAge: time.Minute, ReapLimit: 8,
			RetryInitial: time.Millisecond, RetryMaximum: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewJobService() error = %v", err)
	}
	return service
}

func newTestDurableWorker(
	t *testing.T,
	service *submissionservice.JobService,
	scheduler *FairScheduler,
	executor DurableLeaseExecutor,
) *DurableWorker {
	t.Helper()
	worker, err := NewDurableWorker(
		service,
		scheduler,
		executor,
		DurableWorkerConfig{
			ClaimInterval: time.Hour, ReapInterval: time.Hour,
			WorkerHeartbeatInterval: 10 * time.Millisecond,
			LeaseHeartbeatInterval:  5 * time.Millisecond,
		},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewDurableWorker() error = %v", err)
	}
	return worker
}

func validDurableWorkerLease(
	sequence int,
	lane string,
	leaseDuration time.Duration,
) submissionports.JobLease {
	lease := validScheduledLease(sequence, lane)
	lease.LeaseUntil = time.Now().Add(leaseDuration)
	return lease
}
