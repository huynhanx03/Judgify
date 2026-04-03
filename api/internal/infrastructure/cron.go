package infrastructure

import (
	"context"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	problemPorts "github.com/huynhanx03/judgify/internal/problem/ports"
)

// statsRecalculator tracks the last recalculation time for incremental processing.
type statsRecalculator struct {
	problemRepo problemPorts.ProblemRepository
	logger      *zap.Logger
	mu          sync.Mutex
	lastRun     *time.Time
}

// run recalculates problem stats incrementally since the last run.
func (s *statsRecalculator) run() {
	s.mu.Lock()
	since := s.lastRun
	now := time.Now()
	s.mu.Unlock()

	ctx := context.Background()
	if err := s.problemRepo.RecalculateStats(ctx, since); err != nil {
		s.logger.Error("stats recalculation failed", zap.Error(err))
		return
	}

	s.mu.Lock()
	s.lastRun = &now
	s.mu.Unlock()

	s.logger.Debug("stats recalculation completed", zap.Timep("since", since))
}

// StartScheduler starts the background gocron scheduler.
func StartScheduler(problemRepo problemPorts.ProblemRepository) gocron.Scheduler {
	log := global.LoggerZap.Named("scheduler")

	s, err := gocron.NewScheduler()
	if err != nil {
		log.Fatal("failed to create scheduler", zap.Error(err))
	}

	recalc := &statsRecalculator{
		problemRepo: problemRepo,
		logger:      log,
	}

	// Recalculate problem stats every 5 minutes (incremental since last run)
	_, err = s.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(recalc.run),
	)
	if err != nil {
		log.Fatal("failed to add stats recalculation job", zap.Error(err))
	}

	s.Start()
	log.Info("Scheduler started")
	return s
}
