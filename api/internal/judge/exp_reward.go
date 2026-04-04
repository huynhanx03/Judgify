package judge

import (
	"context"
	"encoding/json"
	"math"

	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	problemPorts "github.com/huynhanx03/judgify/internal/problem/ports"
	"github.com/huynhanx03/judgify/pkg/common/workerpool"
	"github.com/huynhanx03/judgify/pkg/mq/forge"
)

// ExpRewardJob is the MQ message payload for EXP reward events.
type ExpRewardJob struct {
	UserID       int  `json:"user_id"`
	ProblemID    int  `json:"problem_id"`
	IsFirstSolve bool `json:"is_first_solve"`
}

// ExpRewardWorker consumes EXP reward events and updates user cultivation data.
type ExpRewardWorker struct {
	consumer        *forge.Consumer
	pool            *workerpool.GenericPool[ExpRewardJob]
	problemRepo     problemPorts.ProblemRepository
	userStatsRepo   cultivationPorts.UserStatsRepository
	userElemExpRepo cultivationPorts.UserElementExpRepository
	userDiffRepo    cultivationPorts.UserDifficultyStatsRepository
	userTagRepo     cultivationPorts.UserTagStatsRepository
	userTraitRepo   cultivationPorts.UserTraitRepository
	logger          *zap.Logger
	stopCh          chan struct{}
}

// NewExpRewardWorker creates an EXP reward event consumer.
func NewExpRewardWorker(
	consumer *forge.Consumer,
	problemRepo problemPorts.ProblemRepository,
	userStatsRepo cultivationPorts.UserStatsRepository,
	userElemExpRepo cultivationPorts.UserElementExpRepository,
	userDiffRepo cultivationPorts.UserDifficultyStatsRepository,
	userTagRepo cultivationPorts.UserTagStatsRepository,
	userTraitRepo cultivationPorts.UserTraitRepository,
) (*ExpRewardWorker, error) {
	w := &ExpRewardWorker{
		consumer:        consumer,
		problemRepo:     problemRepo,
		userStatsRepo:   userStatsRepo,
		userElemExpRepo: userElemExpRepo,
		userDiffRepo:    userDiffRepo,
		userTagRepo:     userTagRepo,
		userTraitRepo:   userTraitRepo,
		logger:          global.LoggerZap.Named("exp-reward"),
		stopCh:          make(chan struct{}),
	}

	pool, err := workerpool.NewGenericPool[ExpRewardJob](4, w.processReward)
	if err != nil {
		return nil, err
	}
	w.pool = pool

	return w, nil
}

// Start begins polling MQ for EXP reward events.
func (w *ExpRewardWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			default:
				records, err := w.consumer.Poll(10)
				if err != nil {
					w.logger.Error("poll error", zap.Error(err))
					continue
				}
				if len(records) == 0 {
					continue
				}

				for _, rec := range records {
					var job ExpRewardJob
					if err := json.Unmarshal(rec.Value, &job); err != nil {
						w.logger.Error("invalid exp reward payload", zap.Error(err))
						continue
					}
					if err := w.pool.Invoke(job); err != nil {
						w.logger.Error("pool invoke error", zap.Error(err))
					}
				}

				if err := w.consumer.Commit(); err != nil {
					w.logger.Error("commit error", zap.Error(err))
				}
			}
		}
	}()
}

// Stop gracefully stops the worker.
func (w *ExpRewardWorker) Stop() {
	close(w.stopCh)
	w.pool.Release()
}

// expBonus holds EXP bonuses derived from user traits.
// Two layers:
//   - multi*  : percentage multiplier bonus (exp_multiplier trait)
//   - flat*   : flat EXP added after multiplication (exp_bonus trait)
type expBonus struct {
	multiAll    float64            // multiplier bonus for all elements (e.g. 0.3 = +30%)
	multiByElem map[string]float64 // extra multiplier bonus per element code
	flatAll     float64            // flat EXP bonus for all elements
	flatByElem  map[string]float64 // extra flat EXP bonus per element code
}

// calcExpBonus parses user traits and accumulates exp_multiplier and exp_bonus effects.
func calcExpBonus(traits []*cultivationEntity.Trait) expBonus {
	b := expBonus{
		multiByElem: make(map[string]float64),
		flatByElem:  make(map[string]float64),
	}
	for _, t := range traits {
		raw, err := json.Marshal(t.Metadata)
		if err != nil {
			continue
		}
		var effect cultivationEntity.TraitEffect
		if err := json.Unmarshal(raw, &effect); err != nil {
			continue
		}
		switch effect.Type {
		case "exp_multiplier":
			extra := effect.Value - 1 // 1.5 → +0.5
			switch effect.TargetScope {
			case "all":
				b.multiAll += extra
			case "element":
				for _, code := range effect.TargetElements {
					b.multiByElem[code] += extra
				}
			}
		case "exp_bonus":
			switch effect.TargetScope {
			case "all":
				b.flatAll += effect.Value
			case "element":
				for _, code := range effect.TargetElements {
					b.flatByElem[code] += effect.Value
				}
			}
		}
	}
	return b
}

// applyElem returns EXP for a specific element:
//
//	baseExp × (1 + multiAll + multiByElem[code]) + flatAll + flatByElem[code]
func (b expBonus) applyElem(base int64, elemCode string) int64 {
	multiplied := float64(base) * (1.0 + b.multiAll + b.multiByElem[elemCode])
	return int64(math.Round(multiplied + b.flatAll + b.flatByElem[elemCode]))
}

// applyTotal returns total level EXP:
//
//	baseExp × (1 + multiAll + matchMulti) + flatAll + Σ flatByElem[code] for matching elements
//
// matchMulti and matchFlat are pre-summed from the problem's elements.
func (b expBonus) applyTotal(base int64, matchMulti float64, matchFlat float64) int64 {
	multiplied := float64(base) * (1.0 + b.multiAll + matchMulti)
	return int64(math.Round(multiplied + b.flatAll + matchFlat))
}

// processReward handles one EXP reward event.
func (w *ExpRewardWorker) processReward(job ExpRewardJob) {
	ctx := context.Background()
	log := w.logger.With(zap.Int("user_id", job.UserID), zap.Int("problem_id", job.ProblemID))

	// Load problem with difficulty + tags (with elements)
	problem, err := w.problemRepo.Get(ctx, job.ProblemID)
	if err != nil {
		log.Error("problem not found", zap.Error(err))
		return
	}

	if !job.IsFirstSolve {
		return // EXP chỉ thưởng lần đầu solve
	}

	if problem.Difficulty == nil {
		log.Warn("problem has no difficulty, skipping EXP reward")
		return
	}

	baseExp := problem.Difficulty.ExpReward
	if baseExp <= 0 {
		return
	}

	// Load user traits to calculate EXP bonus
	traits, err := w.userTraitRepo.GetByUserID(ctx, job.UserID)
	if err != nil {
		log.Warn("failed to load user traits, using base EXP", zap.Error(err))
		traits = nil
	}
	bonus := calcExpBonus(traits)

	// Collect unique elements of this problem
	type elemEntry struct{ id int; code string }
	var problemElems []elemEntry
	elemSeen := make(map[int]struct{})
	for _, tag := range problem.Tags {
		for _, elem := range tag.Elements {
			if _, seen := elemSeen[elem.ID]; seen {
				continue
			}
			elemSeen[elem.ID] = struct{}{}
			problemElems = append(problemElems, elemEntry{id: elem.ID, code: elem.Code})
		}
	}

	// matchMulti / matchFlat = sum over problem's elements of the user's per-element bonuses.
	// Elements the user has no trait for contribute 0.
	matchMulti := 0.0
	matchFlat := 0.0
	for _, e := range problemElems {
		matchMulti += bonus.multiByElem[e.code]
		matchFlat += bonus.flatByElem[e.code]
	}

	// 1) Total level EXP: baseExp × (1 + multiAll + matchMulti) + flatAll + matchFlat
	totalExp := bonus.applyTotal(baseExp, matchMulti, matchFlat)
	if err := w.userStatsRepo.AddExp(ctx, job.UserID, totalExp); err != nil {
		log.Error("failed to add total EXP", zap.Error(err))
		return
	}

	// 2) Per-element EXP: baseExp × (1 + allBonus + byElem[code])
	//    Water with no trait → byElem["water"]=0 → only gets allBonus, no extra.
	for _, e := range problemElems {
		elemExp := bonus.applyElem(baseExp, e.code)
		if err := w.userElemExpRepo.AddExpByElement(ctx, job.UserID, e.id, elemExp); err != nil {
			log.Error("failed to add element EXP", zap.Int("element_id", e.id), zap.Error(err))
		}
	}

	// 3) Update difficulty stats + tag stats
	if err := w.userDiffRepo.IncrementSolved(ctx, job.UserID, problem.Difficulty.ID); err != nil {
		log.Error("failed to increment difficulty stats", zap.Error(err))
	}

	tagSeen := make(map[int]struct{})
	for _, tag := range problem.Tags {
		if _, seen := tagSeen[tag.ID]; seen {
			continue
		}
		tagSeen[tag.ID] = struct{}{}
		if err := w.userTagRepo.IncrementSolved(ctx, job.UserID, tag.ID); err != nil {
			log.Error("failed to increment tag stats", zap.Int("tag_id", tag.ID), zap.Error(err))
		}
	}

	log.Info("EXP rewarded",
		zap.Int64("base_exp", baseExp),
		zap.Int64("total_exp", totalExp),
		zap.Float64("multi_all", bonus.multiAll),
		zap.Float64("flat_all", bonus.flatAll),
		zap.Float64("match_multi", matchMulti),
		zap.Float64("match_flat", matchFlat),
		zap.Int("elements", len(problemElems)),
	)
}
