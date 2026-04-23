package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/pkg/common/cache"
)

const rankingTTL = 10 * time.Minute

type rankingService struct {
	userStatsRepo ports.UserStatsRepository
	rankRepo      ports.RankRepository
	levelRepo     ports.LevelRepository
	cache         cache.LocalCache[string, any]
	sf            singleflight.Group
}

// NewRankingService creates a new RankingService instance.
func NewRankingService(
	userStatsRepo ports.UserStatsRepository,
	rankRepo ports.RankRepository,
	levelRepo ports.LevelRepository,
	c cache.LocalCache[string, any],
) ports.RankingService {
	return &rankingService{
		userStatsRepo: userStatsRepo,
		rankRepo:      rankRepo,
		levelRepo:     levelRepo,
		cache:         c,
	}
}

// GetTopByRating returns top users sorted by rating with rank titles.
func (s *rankingService) GetTopByRating(ctx context.Context, limit int) ([]*dto.RankingEntry, error) {
	key := fmt.Sprintf("ranking:rating:%d", limit)
	return cache.Fetch[[]*dto.RankingEntry](s.cache, &s.sf, key, rankingTTL, func() ([]*dto.RankingEntry, error) {
		return s.buildRatingEntries(ctx, limit)
	})
}

// GetTopByExp returns top users sorted by total EXP with level names.
func (s *rankingService) GetTopByExp(ctx context.Context, limit int) ([]*dto.RankingEntry, error) {
	key := fmt.Sprintf("ranking:exp:%d", limit)
	return cache.Fetch[[]*dto.RankingEntry](s.cache, &s.sf, key, rankingTTL, func() ([]*dto.RankingEntry, error) {
		return s.buildExpEntries(ctx, limit)
	})
}

func (s *rankingService) buildRatingEntries(ctx context.Context, limit int) ([]*dto.RankingEntry, error) {
	records, err := s.userStatsRepo.GetTopSortedWithUser(ctx, "rating", limit)
	if err != nil {
		return nil, err
	}

	ranks, err := s.rankRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].MinRating > ranks[j].MinRating
	})

	entries := make([]*dto.RankingEntry, len(records))
	for i, r := range records {
		entry := &dto.RankingEntry{
			Rank:     i + 1,
			UserID:   r.Stats.UserID,
			Username: r.Username,
			Rating:   r.Stats.Rating,
			TotalExp: r.Stats.TotalExp,
		}
		for _, rk := range ranks {
			if rk.MinRating <= r.Stats.Rating {
				entry.RankTitle = rk.Name
				break
			}
		}
		entries[i] = entry
	}
	return entries, nil
}

func (s *rankingService) buildExpEntries(ctx context.Context, limit int) ([]*dto.RankingEntry, error) {
	records, err := s.userStatsRepo.GetTopSortedWithUser(ctx, "total_exp", limit)
	if err != nil {
		return nil, err
	}

	levels, err := s.levelRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].MinExp > levels[j].MinExp
	})

	sortedAsc := make([]struct {
		Name   string
		MinExp int64
	}, len(levels))
	for i, j := 0, len(levels)-1; j >= 0; i, j = i+1, j-1 {
		sortedAsc[i].Name = levels[j].Name
		sortedAsc[i].MinExp = levels[j].MinExp
	}

	entries := make([]*dto.RankingEntry, len(records))
	for i, r := range records {
		entry := &dto.RankingEntry{
			Rank:     i + 1,
			UserID:   r.Stats.UserID,
			Username: r.Username,
			Rating:   r.Stats.Rating,
			TotalExp: r.Stats.TotalExp,
		}
		for idx, lv := range sortedAsc {
			if lv.MinExp <= r.Stats.TotalExp {
				entry.LevelName = lv.Name
				entry.Level = idx + 1
				break
			}
		}
		entries[i] = entry
	}
	return entries, nil
}
