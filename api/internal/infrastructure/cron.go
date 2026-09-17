package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	problemPorts "github.com/huynhanx03/judgify/internal/ports"
)

var errSchedulerInvalidState = errors.New("scheduler: invalid lifecycle state")

const defaultStatsInterval = 5 * time.Minute

type schedulerState uint8

const (
	schedulerNew schedulerState = iota
	schedulerRunning
	schedulerStopping
	schedulerStopped
)

// statsRecalculator tracks the last successful incremental recalculation.
type statsRecalculator struct {
	problemRepo problemPorts.ProblemRepository
	mu          sync.Mutex
	lastRun     *time.Time
}

func (recalculator *statsRecalculator) run(ctx context.Context) {
	recalculator.mu.Lock()
	since := recalculator.lastRun
	recalculator.mu.Unlock()

	// Capture the upper watermark before the query starts. Writes committed
	// during the scan remain strictly newer and are therefore picked up by the
	// next run instead of falling into a query/update race window.
	now := time.Now().UTC()
	if err := recalculator.problemRepo.RecalculateStats(ctx, since); err != nil {
		if ctx.Err() == nil {
			commonlogger.FromContext(ctx).Error("stats recalculation failed", zap.Error(err))
		}
		return
	}
	recalculator.mu.Lock()
	recalculator.lastRun = &now
	recalculator.mu.Unlock()
	commonlogger.FromContext(ctx).Debug("stats recalculation completed", zap.Timep("since", since))
}

// Scheduler is a lifecycle-owned, single-loop stats scheduler. It deliberately
// avoids a third-party background runtime so every job inherits the lifecycle
// context and Shutdown can honor the exact remaining component budget.
type Scheduler struct {
	problemRepo problemPorts.ProblemRepository
	logger      *zap.Logger
	interval    time.Duration

	mu         sync.Mutex
	state      schedulerState
	runCancel  context.CancelFunc
	runDone    chan struct{}
	jobRunning bool
	work       errgroup.Group
	waiter     errgroup.Group
	waitOnce   sync.Once
	workDone   chan struct{}
	workErr    error
}

// NewScheduler validates and stores dependencies without starting work.
func NewScheduler(problemRepo problemPorts.ProblemRepository, logger *zap.Logger) (*Scheduler, error) {
	if problemRepo == nil || logger == nil {
		return nil, fmt.Errorf("scheduler: invalid dependencies")
	}
	return &Scheduler{
		problemRepo: problemRepo,
		logger:      logger,
		interval:    defaultStatsInterval,
		state:       schedulerNew,
		runDone:     make(chan struct{}),
		workDone:    make(chan struct{}),
	}, nil
}

func (*Scheduler) Name() string { return "stats-scheduler" }

func (scheduler *Scheduler) Run(ctx context.Context) error {
	if ctx == nil {
		return errSchedulerInvalidState
	}
	scheduler.mu.Lock()
	if scheduler.state != schedulerNew {
		scheduler.mu.Unlock()
		return errSchedulerInvalidState
	}
	runCtx, cancel := context.WithCancel(
		commonlogger.WithContext(ctx, scheduler.logger),
	)
	scheduler.runCancel = cancel
	scheduler.state = schedulerRunning
	scheduler.mu.Unlock()

	recalculator := &statsRecalculator{problemRepo: scheduler.problemRepo}
	ticker := time.NewTicker(scheduler.interval)
	defer ticker.Stop()
	defer func() {
		cancel()
		scheduler.mu.Lock()
		close(scheduler.runDone)
		scheduler.mu.Unlock()
	}()

	log := commonlogger.FromContext(runCtx)
	log.Info("stats scheduler started")
	for {
		select {
		case <-runCtx.Done():
			log.Info("stats scheduler stopped")
			return nil
		case <-ticker.C:
			scheduler.startJob(runCtx, recalculator)
		}
	}
}

func (scheduler *Scheduler) startJob(ctx context.Context, recalculator *statsRecalculator) {
	scheduler.mu.Lock()
	if scheduler.jobRunning || scheduler.state != schedulerRunning {
		scheduler.mu.Unlock()
		return
	}
	scheduler.jobRunning = true
	scheduler.mu.Unlock()
	scheduler.work.Go(func() error {
		defer func() {
			scheduler.mu.Lock()
			scheduler.jobRunning = false
			scheduler.mu.Unlock()
		}()
		recalculator.run(ctx)
		return nil
	})
}

func (scheduler *Scheduler) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errSchedulerInvalidState
	}
	scheduler.mu.Lock()
	switch scheduler.state {
	case schedulerNew:
		scheduler.state = schedulerStopped
		close(scheduler.runDone)
		scheduler.mu.Unlock()
		return nil
	case schedulerStopped:
		scheduler.mu.Unlock()
		return nil
	case schedulerRunning:
		scheduler.state = schedulerStopping
		scheduler.runCancel()
	case schedulerStopping:
	}
	done := scheduler.runDone
	scheduler.mu.Unlock()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	scheduler.waitOnce.Do(func() {
		scheduler.waiter.Go(func() error {
			scheduler.workErr = scheduler.work.Wait()
			close(scheduler.workDone)
			return nil
		})
	})
	select {
	case <-scheduler.workDone:
		scheduler.mu.Lock()
		scheduler.state = schedulerStopped
		scheduler.mu.Unlock()
		return scheduler.workErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
