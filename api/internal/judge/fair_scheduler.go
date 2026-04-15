package judge

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/pkg/mq/forge"
)

// FairScheduler polls two MQ consumers with weighted priority.
// Contest jobs get contestWeight shares, regular jobs get regularWeight shares.
type FairScheduler struct {
	contestConsumer *forge.Consumer
	regularConsumer *forge.Consumer
	contestWeight   int // e.g. 80
	regularWeight   int // e.g. 20
	dispatch        func(JudgeJob)
	logger          *zap.Logger
}

// NewFairScheduler creates a weighted round-robin scheduler for two consumers.
func NewFairScheduler(
	contestConsumer, regularConsumer *forge.Consumer,
	contestWeight, regularWeight int,
	dispatch func(JudgeJob),
	logger *zap.Logger,
) *FairScheduler {
	return &FairScheduler{
		contestConsumer: contestConsumer,
		regularConsumer: regularConsumer,
		contestWeight:   contestWeight,
		regularWeight:   regularWeight,
		dispatch:        dispatch,
		logger:          logger,
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled or stopCh is closed.
func (s *FairScheduler) Run(ctx context.Context, stopCh chan struct{}) {
	totalWeight := s.contestWeight + s.regularWeight
	contestBudget := s.contestWeight
	regularBudget := s.regularWeight

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		default:
		}

		consumed := 0

		// Contest queue (high priority)
		if contestBudget > 0 {
			n := s.pollAndDispatch(s.contestConsumer, contestBudget, "contest")
			contestBudget -= n
			consumed += n
		}

		// Regular queue
		if regularBudget > 0 {
			n := s.pollAndDispatch(s.regularConsumer, regularBudget, "regular")
			regularBudget -= n
			consumed += n
		}

		// Reset budgets when both depleted
		if contestBudget <= 0 && regularBudget <= 0 {
			contestBudget = s.contestWeight
			regularBudget = s.regularWeight
		}

		// If nothing consumed from either, steal budget from empty queue
		if consumed == 0 {
			if contestBudget <= 0 {
				regularBudget = totalWeight // regular gets all
			} else {
				contestBudget = totalWeight // contest gets all
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// pollAndDispatch polls a consumer and dispatches jobs. Returns count consumed.
func (s *FairScheduler) pollAndDispatch(consumer *forge.Consumer, budget int, label string) int {
	if budget <= 0 {
		return 0
	}

	limit := budget
	if limit > 10 {
		limit = 10
	}

	records, err := consumer.Poll(limit)
	if err != nil {
		s.logger.Error("poll error", zap.String("queue", label), zap.Error(err))
		return 0
	}

	if len(records) == 0 {
		return 0
	}

	for _, rec := range records {
		var job JudgeJob
		if err := json.Unmarshal(rec.Value, &job); err != nil {
			s.logger.Error("invalid job payload", zap.String("queue", label), zap.Error(err))
			continue
		}
		s.dispatch(job)
	}

	if err := consumer.Commit(); err != nil {
		s.logger.Error("commit error", zap.String("queue", label), zap.Error(err))
	}

	return len(records)
}
