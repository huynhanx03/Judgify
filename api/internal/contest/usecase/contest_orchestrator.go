package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"

	"github.com/huynhanx03/judgify/internal/constant"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

var (
	errContestOrchestratorInvalidState = errors.New("contest orchestrator: invalid lifecycle state")
	errContestOrchestratorNilContext   = errors.New("contest orchestrator: context is nil")
)

type contestOrchestratorState uint8

const (
	contestOrchestratorNew contestOrchestratorState = iota
	contestOrchestratorRunning
	contestOrchestratorStopping
	contestOrchestratorStopped
)

// ContestOrchestrator periodically transitions contest statuses based on time.
type ContestOrchestrator struct {
	contestRepo    ports.ContestRepository
	ratingSvc      ports.RatingService
	settlementRepo ports.RatingSettlementRepository
	lifecycle      ports.LifecycleService
	logger         *zap.Logger
	tickInterval   time.Duration

	mu        sync.Mutex
	state     contestOrchestratorState
	runCancel context.CancelFunc
	runDone   chan struct{}
}

// NewContestOrchestrator creates a new ContestOrchestrator.
func NewContestOrchestrator(
	contestRepo ports.ContestRepository,
	ratingSvc ports.RatingService,
	settlementRepo ports.RatingSettlementRepository,
	log *zap.Logger,
	lifecycleServices ...ports.LifecycleService,
) *ContestOrchestrator {
	if log == nil {
		log = zap.NewNop()
	}
	var lifecycleService ports.LifecycleService
	if len(lifecycleServices) == 1 {
		lifecycleService = lifecycleServices[0]
	}
	return &ContestOrchestrator{
		contestRepo:    contestRepo,
		ratingSvc:      ratingSvc,
		settlementRepo: settlementRepo,
		lifecycle:      lifecycleService,
		logger:         log,
		tickInterval:   constant.ContestOrchestratorTickInterval,
		state:          contestOrchestratorNew,
		runDone:        make(chan struct{}),
	}
}

// Name identifies the component in the application lifecycle.
func (*ContestOrchestrator) Name() string {
	return "contest-orchestrator"
}

// Run executes the status transition loop until the caller cancels ctx or
// Shutdown requests termination. A ContestOrchestrator can be run once.
func (o *ContestOrchestrator) Run(ctx context.Context) error {
	if ctx == nil {
		return errContestOrchestratorNilContext
	}

	o.mu.Lock()
	if o.state != contestOrchestratorNew {
		o.mu.Unlock()
		return errContestOrchestratorInvalidState
	}
	runCtx, cancel := context.WithCancel(
		commonlogger.WithContext(ctx, o.logger),
	)
	o.runCancel = cancel
	o.state = contestOrchestratorRunning
	o.mu.Unlock()

	ticker := time.NewTicker(o.tickInterval)
	defer ticker.Stop()
	defer func() {
		cancel()
		o.mu.Lock()
		o.state = contestOrchestratorStopped
		close(o.runDone)
		o.mu.Unlock()
	}()

	log := commonlogger.FromContext(runCtx)
	log.Info("orchestrator started")
	// Reconcile persisted lifecycle and settlement state immediately after a
	// restart; correctness must not wait for the first periodic tick.
	o.transitionStatuses(runCtx)
	if runCtx.Err() != nil {
		log.Info("orchestrator stopped")
		return nil
	}
	for {
		select {
		case <-runCtx.Done():
			log.Info("orchestrator stopped")
			return nil
		case <-ticker.C:
			o.transitionStatuses(runCtx)
			if runCtx.Err() != nil {
				log.Info("orchestrator stopped")
				return nil
			}
		}
	}
}

// Shutdown requests termination and waits for Run to release its owned work.
// It is safe to call concurrently or repeatedly and honors ctx's deadline.
func (o *ContestOrchestrator) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errContestOrchestratorNilContext
	}

	o.mu.Lock()
	switch o.state {
	case contestOrchestratorNew:
		o.state = contestOrchestratorStopped
		close(o.runDone)
		o.mu.Unlock()
		return nil
	case contestOrchestratorStopped:
		o.mu.Unlock()
		return nil
	case contestOrchestratorRunning:
		o.state = contestOrchestratorStopping
		o.runCancel()
	case contestOrchestratorStopping:
	}
	done := o.runDone
	o.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// transitionStatuses checks upcoming/running contests and updates their status.
func (o *ContestOrchestrator) transitionStatuses(ctx context.Context) {
	now := time.Now()
	log := commonlogger.FromContext(ctx)

	upcomingIDs, err := o.contestRepo.FindIDsByStatus(ctx, constant.ContestStatusUpcoming)
	if err != nil {
		log.Error("failed to fetch upcoming contests", zap.Error(err))
	}
	for _, id := range upcomingIDs {
		contestCtx := contestOperationContext(ctx, id)
		contestLog := commonlogger.FromContext(contestCtx)
		c, err := o.contestRepo.Get(contestCtx, id)
		if err != nil {
			contestLog.Error("failed to get contest", zap.Error(err))
			continue
		}
		// A delayed scheduler must not leave a contest in upcoming after its
		// entire window has elapsed. The lifecycle command validates this
		// against database time before accepting the system-only expiry path.
		if !c.EndTime.IsZero() && !c.EndTime.After(now) {
			if err := o.transitionSystem(contestCtx, c, entity.ContestEnded); err != nil {
				contestLog.Error("failed to transition expired upcoming contest to ended through lifecycle command", zap.Error(err))
				continue
			}
			contestLog.Info("expired upcoming contest ended")
			continue
		}
		if !c.StartTime.After(now) {
			if err := o.transitionSystem(contestCtx, c, entity.ContestRunning); err != nil {
				contestLog.Error("failed to transition to running through lifecycle command", zap.Error(err))
				continue
			}
			contestLog.Info("contest started")
		}
	}

	runningIDs, err := o.contestRepo.FindIDsByStatus(ctx, constant.ContestStatusRunning)
	if err != nil {
		log.Error("failed to fetch running contests", zap.Error(err))
	}
	for _, id := range runningIDs {
		contestCtx := contestOperationContext(ctx, id)
		contestLog := commonlogger.FromContext(contestCtx)
		c, err := o.contestRepo.Get(contestCtx, id)
		if err != nil {
			contestLog.Error("failed to get contest", zap.Error(err))
			continue
		}
		if !c.EndTime.After(now) {
			if err := o.transitionSystem(contestCtx, c, entity.ContestEnded); err != nil {
				contestLog.Error("failed to transition to ended through lifecycle command", zap.Error(err))
				continue
			}
			contestLog.Info("contest ended")

		}
	}

	o.settlePendingRatings(ctx)
}

const systemContestLifecycleReason = "automatic schedule transition"

// transitionSystem is intentionally a command, never a status write. The
// lifecycle repository locks the aggregate, uses PostgreSQL time, freezes
// publish membership when applicable, and appends transition/audit/outbox in
// the same UnitOfWork. The system flag is explicit evidence that no human
// principal performed the transition.
func (o *ContestOrchestrator) transitionSystem(
	ctx context.Context,
	contest *entity.Contest,
	target entity.ContestState,
) error {
	if ctx == nil || contest == nil || contest.ID == uuid.Nil || !target.Valid() {
		return errors.New("contest orchestrator: invalid automatic lifecycle command")
	}
	if o.lifecycle == nil {
		return errors.New("contest orchestrator: lifecycle service unavailable")
	}
	expectedVersion := contest.Version
	if expectedVersion < 1 {
		return errors.New("contest orchestrator: invalid contest version")
	}
	commandID := uuid.NewSHA1(
		uuid.NameSpaceOID,
		[]byte(fmt.Sprintf("judgify/contest/system-transition/v1/%s/%s/%d", contest.ID, target, expectedVersion)),
	)
	_, err := o.lifecycle.Transition(ctx, ports.TransitionCommand{
		ContestID: contest.ID, Target: target, ExpectedVersion: expectedVersion,
		IdempotencyKey: commandID, System: true, Reason: systemContestLifecycleReason,
	})
	return err
}

const pendingRatingBatchSize = 100

func (o *ContestOrchestrator) settlePendingRatings(ctx context.Context) {
	log := commonlogger.FromContext(ctx)
	if o.settlementRepo == nil || o.ratingSvc == nil {
		log.Error("rating settlement dependencies are unavailable")
		return
	}

	contestIDs, err := o.settlementRepo.FindPending(ctx, pendingRatingBatchSize)
	if err != nil {
		log.Error("failed to fetch pending rating settlements", zap.Error(err))
		return
	}
	for _, contestID := range contestIDs {
		if ctx.Err() != nil {
			return
		}
		contestCtx := contestOperationContext(ctx, contestID)
		contestLog := commonlogger.FromContext(contestCtx)
		if err := o.ratingSvc.CalculateRating(contestCtx, contestID); err != nil {
			if errors.Is(err, ports.ErrRatingSequenceBlocked) ||
				errors.Is(err, ports.ErrRatingSettlementNotReady) {
				contestLog.Info(
					"rating sequence is waiting for final scoring",
					zap.Error(err),
				)
				break
			}
			contestLog.Error("failed to settle contest rating", zap.Error(err))
			continue
		}
		contestLog.Info("contest rating settled")
	}
}

func contestOperationContext(ctx context.Context, contestID uuid.UUID) context.Context {
	return commonlogger.WithFields(
		ctx,
		commonlogger.String("contest_id", contestID.String()),
	)
}
