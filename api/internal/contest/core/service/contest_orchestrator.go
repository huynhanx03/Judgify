package service

import (
	"context"
	"time"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/contest/constant"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	"go.uber.org/zap"
)

// ContestOrchestrator periodically transitions contest statuses based on time.
type ContestOrchestrator struct {
	contestRepo ports.ContestRepository
	ratingSvc   ports.RatingService
	logger      *zap.Logger
	stopCh      chan struct{}
}

// NewContestOrchestrator creates a new ContestOrchestrator.
func NewContestOrchestrator(contestRepo ports.ContestRepository, ratingSvc ports.RatingService) *ContestOrchestrator {
	return &ContestOrchestrator{
		contestRepo: contestRepo,
		ratingSvc:   ratingSvc,
		logger:      global.LoggerZap.Named("contest-orchestrator"),
		stopCh:      make(chan struct{}),
	}
}

// Start launches the status transition loop. Blocks until Stop is called.
func (o *ContestOrchestrator) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	o.logger.Info("orchestrator started")
	for {
		select {
		case <-o.stopCh:
			o.logger.Info("orchestrator stopped")
			return
		case <-ctx.Done():
			o.logger.Info("orchestrator stopped via context")
			return
		case <-ticker.C:
			o.transitionStatuses(ctx)
		}
	}
}

// Stop signals the orchestrator to stop.
func (o *ContestOrchestrator) Stop() {
	close(o.stopCh)
}

// transitionStatuses checks upcoming/running contests and updates their status.
func (o *ContestOrchestrator) transitionStatuses(ctx context.Context) {
	now := time.Now()

	upcomingIDs, err := o.contestRepo.FindIDsByStatus(ctx, constant.StatusUpcoming)
	if err != nil {
		o.logger.Error("failed to fetch upcoming contests", zap.Error(err))
	}
	for _, id := range upcomingIDs {
		c, err := o.contestRepo.Get(ctx, id)
		if err != nil {
			o.logger.Error("failed to get contest", zap.Int("contest_id", id), zap.Error(err))
			continue
		}
		if !c.StartTime.After(now) {
			if err := o.contestRepo.UpdateStatus(ctx, id, constant.StatusRunning); err != nil {
				o.logger.Error("failed to transition to running", zap.Int("contest_id", id), zap.Error(err))
				continue
			}
			o.logger.Info("contest started", zap.Int("contest_id", id))
		}
	}

	runningIDs, err := o.contestRepo.FindIDsByStatus(ctx, constant.StatusRunning)
	if err != nil {
		o.logger.Error("failed to fetch running contests", zap.Error(err))
	}
	for _, id := range runningIDs {
		c, err := o.contestRepo.Get(ctx, id)
		if err != nil {
			o.logger.Error("failed to get contest", zap.Int("contest_id", id), zap.Error(err))
			continue
		}
		if !c.EndTime.After(now) {
			if err := o.contestRepo.UpdateStatus(ctx, id, constant.StatusEnded); err != nil {
				o.logger.Error("failed to transition to ended", zap.Int("contest_id", id), zap.Error(err))
				continue
			}
			o.logger.Info("contest ended", zap.Int("contest_id", id))

			// Calculate Elo rating changes asynchronously.
			if o.ratingSvc != nil {
				go func() {
					if err := o.ratingSvc.CalculateRating(context.Background(), id); err != nil {
						o.logger.Error("failed to calculate rating", zap.Int("contest_id", id), zap.Error(err))
					}
				}()
			}
		}
	}
}
