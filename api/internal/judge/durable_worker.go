package judge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	submissionports "github.com/huynhanx03/judgify/internal/ports"
	submissionservice "github.com/huynhanx03/judgify/internal/submission/usecase"
)

const consecutiveHeartbeatFailureLimit = 3

var (
	ErrDurableWorkerInvalid   = errors.New("durable judge worker: invalid operation")
	errLeaseHardExpired       = errors.New("durable judge worker: lease hard expiry reached")
	errLeaseHeartbeatTimedOut = errors.New("durable judge worker: lease heartbeat timed out")
)

type leaseHeartbeatResult struct {
	acceptedUntil time.Time
	err           error
}

// DurableLeaseExecutor consumes one lease already owned by the local
// scheduler. The lease capability remains valid only while its context is
// alive and the worker's heartbeat loop continues to renew it.
type DurableLeaseExecutor interface {
	ExecuteLease(context.Context, submissionports.JobLease) error
}

type DurableWorkerConfig struct {
	ClaimInterval           time.Duration
	ReapInterval            time.Duration
	WorkerHeartbeatInterval time.Duration
	LeaseHeartbeatInterval  time.Duration
}

// DurableWorker connects PostgreSQL-owned leases to the bounded local
// scheduler. It is deliberately one-shot: restarting a stopped worker
// requires a new scheduler and a new worker boot identity.
type DurableWorker struct {
	jobs      *submissionservice.JobService
	scheduler *FairScheduler
	executor  DurableLeaseExecutor
	config    DurableWorkerConfig
	logger    *zap.Logger

	lifecycleMu sync.Mutex
	started     bool
}

func NewDurableWorker(
	jobs *submissionservice.JobService,
	scheduler *FairScheduler,
	executor DurableLeaseExecutor,
	config DurableWorkerConfig,
	logger *zap.Logger,
) (*DurableWorker, error) {
	if jobs == nil || scheduler == nil ||
		nilLeaseExecutionDependency(executor) || logger == nil ||
		config.ClaimInterval <= 0 || config.ReapInterval <= 0 ||
		config.WorkerHeartbeatInterval <= 0 ||
		config.LeaseHeartbeatInterval <= 0 ||
		config.WorkerHeartbeatInterval > jobs.HeartbeatFreshness()/2 ||
		config.LeaseHeartbeatInterval > jobs.LeaseDuration()/4 {
		return nil, ErrDurableWorkerInvalid
	}
	return &DurableWorker{
		jobs: jobs, scheduler: scheduler, executor: executor,
		config: config, logger: logger,
	}, nil
}

// Run blocks until the caller stops the worker or a worker-boot heartbeat
// proves the process can no longer safely own leases. Cancellation is
// graceful: the scheduler stops admission, waits for all in-flight handlers,
// and wipes every raw lease capability before Run returns.
func (worker *DurableWorker) Run(ctx context.Context) error {
	if worker == nil || ctx == nil {
		return ErrDurableWorkerInvalid
	}
	worker.lifecycleMu.Lock()
	if worker.started {
		worker.lifecycleMu.Unlock()
		return ErrDurableWorkerInvalid
	}
	worker.started = true
	worker.lifecycleMu.Unlock()
	defer worker.scheduler.Close()
	if ctx.Err() != nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(
		commonlogger.WithContext(ctx, worker.logger),
	)
	defer cancel()
	fatal := make(chan error, 1)

	if err := worker.establishInitialWorkerHeartbeat(runCtx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	log := commonlogger.FromContext(runCtx)
	log.Info(
		"durable judge worker started",
		zap.Duration("claim_interval", worker.config.ClaimInterval),
		zap.Duration("reap_interval", worker.config.ReapInterval),
		zap.Duration("worker_heartbeat_interval", worker.config.WorkerHeartbeatInterval),
		zap.Duration("lease_heartbeat_interval", worker.config.LeaseHeartbeatInterval),
	)
	defer log.Info("durable judge worker stopped")

	var components sync.WaitGroup
	components.Add(4)
	go func() {
		defer components.Done()
		if err := worker.scheduler.Run(runCtx, func(
			handlerCtx context.Context,
			lease submissionports.JobLease,
		) {
			worker.executeLease(handlerCtx, lease, fatal)
		}); err != nil && runCtx.Err() == nil {
			worker.signalFatal(fatal, errors.Join(ErrDurableWorkerInvalid, err))
		}
	}()
	go func() {
		defer components.Done()
		worker.runClaimLoop(runCtx)
	}()
	go func() {
		defer components.Done()
		worker.runReapLoop(runCtx)
	}()
	go func() {
		defer components.Done()
		worker.runWorkerHeartbeatLoop(
			runCtx,
			0,
			fatal,
		)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-fatal:
	}
	cancel()
	components.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return runErr
}

// establishInitialWorkerHeartbeat keeps every admission path closed until the
// current boot proves durable liveness. A transient startup failure is retried
// within the same bounded policy as the rolling heartbeat loop.
func (worker *DurableWorker) establishInitialWorkerHeartbeat(
	ctx context.Context,
) error {
	for failures := 1; failures <= consecutiveHeartbeatFailureLimit; failures++ {
		err := worker.jobs.HeartbeatWorkerBoot(ctx)
		if err == nil {
			if failures > 1 {
				commonlogger.FromContext(ctx).Info(
					"durable judge initial worker heartbeat recovered",
					zap.Int("attempt", failures),
				)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		worker.logWorkerHeartbeatFailure(ctx, err, failures)
		if definitiveWorkerBootFailure(err) ||
			failures == consecutiveHeartbeatFailureLimit {
			return err
		}
		timer := time.NewTimer(worker.config.WorkerHeartbeatInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ErrDurableWorkerInvalid
}

func (worker *DurableWorker) runClaimLoop(ctx context.Context) {
	worker.claimAndSchedule(ctx)
	ticker := time.NewTicker(worker.config.ClaimInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.claimAndSchedule(ctx)
		}
	}
}

func (worker *DurableWorker) claimAndSchedule(ctx context.Context) {
	claimed, err := worker.jobs.ClaimAndSchedule(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, submissionservice.ErrJobAdmissionClosed) {
			commonlogger.FromContext(ctx).Debug("durable judge claim skipped by local admission")
			return
		}
		commonlogger.FromContext(ctx).Warn("durable judge claim failed", zap.Error(err))
		return
	}
	if claimed > 0 {
		commonlogger.FromContext(ctx).Debug(
			"durable judge leases scheduled",
			zap.Int("claimed", claimed),
			zap.Int("local_depth", worker.scheduler.Depth()),
		)
	}
}

func (worker *DurableWorker) runReapLoop(ctx context.Context) {
	worker.reapExpired(ctx)
	ticker := time.NewTicker(worker.config.ReapInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.reapExpired(ctx)
		}
	}
}

func (worker *DurableWorker) reapExpired(ctx context.Context) {
	result, err := worker.jobs.ReapExpired(ctx)
	if err != nil {
		if ctx.Err() == nil {
			commonlogger.FromContext(ctx).Warn("durable judge lease reaping failed", zap.Error(err))
		}
		return
	}
	if result.Promoted != 0 || result.Retried != 0 ||
		result.Dead != 0 || result.Cancelled != 0 {
		commonlogger.FromContext(ctx).Info(
			"durable judge leases reaped",
			zap.Int("promoted", result.Promoted),
			zap.Int("retried", result.Retried),
			zap.Int("dead", result.Dead),
			zap.Int("cancelled", result.Cancelled),
		)
	}
}

func (worker *DurableWorker) runWorkerHeartbeatLoop(
	ctx context.Context,
	failures int,
	fatal chan<- error,
) {
	ticker := time.NewTicker(worker.config.WorkerHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := worker.jobs.HeartbeatWorkerBoot(ctx)
			if err == nil {
				failures = 0
				continue
			}
			if ctx.Err() != nil {
				return
			}
			failures++
			worker.logWorkerHeartbeatFailure(ctx, err, failures)
			if definitiveWorkerBootFailure(err) ||
				failures >= consecutiveHeartbeatFailureLimit {
				worker.signalFatal(fatal, err)
				return
			}
		}
	}
}

func (worker *DurableWorker) logWorkerHeartbeatFailure(
	ctx context.Context,
	err error,
	failures int,
) {
	commonlogger.FromContext(ctx).Warn(
		"durable judge worker heartbeat failed",
		zap.Int("consecutive_failures", failures),
		zap.Int("failure_limit", consecutiveHeartbeatFailureLimit),
		zap.Error(err),
	)
}

func (worker *DurableWorker) executeLease(
	parent context.Context,
	lease submissionports.JobLease,
	fatal chan<- error,
) {
	leaseCtx := leaseLogContext(parent, lease)
	leaseCtx = commonlogger.WithContext(leaseCtx, worker.logger)
	executionCtx, cancelExecution := context.WithCancel(leaseCtx)
	leaseDeadlineUpdates := make(chan time.Time, 1)
	heartbeatDone := make(chan error, 1)
	expiryWatchdogDone := make(chan error, 1)
	go func() {
		heartbeatDone <- worker.runLeaseHeartbeatLoop(
			executionCtx,
			cancelExecution,
			lease,
			leaseDeadlineUpdates,
		)
	}()
	go func() {
		expiryWatchdogDone <- worker.runLeaseExpiryWatchdog(
			executionCtx,
			cancelExecution,
			lease,
			leaseDeadlineUpdates,
		)
	}()

	executionErr := worker.executor.ExecuteLease(executionCtx, lease)
	cancelExecution()
	heartbeatErr := <-heartbeatDone
	expiryErr := <-expiryWatchdogDone
	leaseGuardErr := errors.Join(heartbeatErr, expiryErr)

	log := commonlogger.FromContext(leaseCtx)
	if errors.Is(leaseGuardErr, submissionports.ErrWorkerBootLost) ||
		errors.Is(executionErr, submissionports.ErrWorkerBootLost) {
		worker.signalFatal(fatal, submissionports.ErrWorkerBootLost)
	}
	if errors.Is(leaseGuardErr, submissionports.ErrLeaseCancelled) ||
		errors.Is(executionErr, submissionports.ErrLeaseCancelled) {
		acknowledgementCtx, cancelAcknowledgement := context.WithTimeout(
			context.WithoutCancel(leaseCtx),
			worker.jobs.LeaseDuration(),
		)
		err := worker.jobs.AcknowledgeCancellation(acknowledgementCtx, lease)
		cancelAcknowledgement()
		if err != nil {
			log.Error(
				"durable judge cancellation acknowledgement failed",
				zap.Error(err),
			)
		}
		return
	}
	if leaseGuardErr != nil && leaseCtx.Err() == nil {
		log.Warn(
			"durable judge lease guard stopped execution",
			zap.Error(leaseGuardErr),
		)
	}
	if executionErr != nil && !errors.Is(executionErr, context.Canceled) &&
		!errors.Is(executionErr, context.DeadlineExceeded) {
		log.Warn(
			"durable judge lease execution stopped",
			zap.String("error_type", fmt.Sprintf("%T", executionErr)),
		)
	}
}

func (worker *DurableWorker) runLeaseHeartbeatLoop(
	ctx context.Context,
	cancelExecution context.CancelFunc,
	lease submissionports.JobLease,
	leaseDeadlineUpdates chan time.Time,
) error {
	ticker := time.NewTicker(worker.config.LeaseHeartbeatInterval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			acceptedUntil, err := worker.heartbeatLease(ctx, lease)
			if err == nil {
				if acceptedUntil.IsZero() || !acceptedUntil.After(time.Now()) {
					cancelExecution()
					return errors.Join(
						submissionports.ErrLeaseLost,
						errLeaseHardExpired,
					)
				}
				failures = 0
				publishLeaseDeadline(ctx, leaseDeadlineUpdates, acceptedUntil)
				continue
			}
			if definitiveLeaseHeartbeatFailure(err) {
				cancelExecution()
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			failures++
			if failures >= consecutiveHeartbeatFailureLimit {
				cancelExecution()
				return err
			}
			commonlogger.FromContext(ctx).Warn(
				"durable judge lease heartbeat failed",
				zap.Int("consecutive_failures", failures),
				zap.Int("failure_limit", consecutiveHeartbeatFailureLimit),
				zap.Error(err),
			)
		}
	}
}

func (worker *DurableWorker) heartbeatLease(
	ctx context.Context,
	lease submissionports.JobLease,
) (time.Time, error) {
	timeout := worker.config.LeaseHeartbeatInterval / 2
	if timeout <= 0 {
		timeout = time.Nanosecond
	}
	heartbeatCtx, cancelHeartbeat := context.WithTimeout(ctx, timeout)
	defer cancelHeartbeat()
	result := make(chan leaseHeartbeatResult, 1)
	go func() {
		acceptedUntil, err := worker.jobs.Heartbeat(heartbeatCtx, lease)
		result <- leaseHeartbeatResult{
			acceptedUntil: acceptedUntil,
			err:           err,
		}
	}()
	select {
	case heartbeat := <-result:
		return heartbeat.acceptedUntil, heartbeat.err
	case <-heartbeatCtx.Done():
		// Prefer a dependency result already available at the deadline so a
		// definitive cancellation/fence is never hidden by context teardown.
		select {
		case heartbeat := <-result:
			return heartbeat.acceptedUntil, heartbeat.err
		default:
		}
		if ctx.Err() != nil {
			return time.Time{}, ctx.Err()
		}
		return time.Time{}, errors.Join(
			errLeaseHeartbeatTimedOut,
			heartbeatCtx.Err(),
		)
	}
}

func publishLeaseDeadline(
	ctx context.Context,
	updates chan time.Time,
	deadline time.Time,
) {
	select {
	case updates <- deadline.UTC():
		return
	default:
	}
	// Keep only the newest DB-accepted deadline; the watchdog does not need to
	// observe intermediate renewals.
	select {
	case <-updates:
	default:
	}
	select {
	case updates <- deadline.UTC():
	case <-ctx.Done():
	}
}

func (worker *DurableWorker) runLeaseExpiryWatchdog(
	ctx context.Context,
	cancelExecution context.CancelFunc,
	lease submissionports.JobLease,
	leaseDeadlineUpdates <-chan time.Time,
) error {
	deadline := lease.LeaseUntil.UTC()
	timer := time.NewTimer(durationUntil(deadline))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case nextDeadline := <-leaseDeadlineUpdates:
			deadline = nextDeadline.UTC()
			resetTimer(timer, durationUntil(deadline))
		case <-timer.C:
			cancelExecution()
			return errors.Join(
				submissionports.ErrLeaseLost,
				errLeaseHardExpired,
			)
		}
	}
}

func durationUntil(deadline time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func definitiveWorkerBootFailure(err error) bool {
	return errors.Is(err, submissionports.ErrWorkerBootLost)
}

func definitiveLeaseHeartbeatFailure(err error) bool {
	return errors.Is(err, submissionports.ErrLeaseLost) ||
		errors.Is(err, submissionports.ErrLeaseCancelled) ||
		errors.Is(err, submissionports.ErrWorkerBootLost) ||
		errors.Is(err, errLeaseHeartbeatTimedOut)
}

func (*DurableWorker) signalFatal(fatal chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case fatal <- err:
	default:
	}
}

var _ DurableLeaseExecutor = (*LeaseExecutor)(nil)
