package service

import (
	"context"
	"math"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/pkg/logger"
	"go.uber.org/zap"
)

const (
	// minContestsForK20 is the number of contests a user needs before K drops to 20.
	minContestsForK20 = 5
	// kFactorNew is the K-factor for users with fewer than 5 rated contests.
	kFactorNew = 40.0
	// kFactorEstablished is the K-factor for experienced users.
	kFactorEstablished = 20.0
	// eloDivisor is the Elo divisor constant.
	eloDivisor = 400.0
)

type ratingService struct {
	standingRepo ports.StandingRepository
	ratingRepo   ports.RatingHistoryRepository
	userStatsRepo cultivationPorts.UserStatsRepository
}

// NewRatingService creates a new RatingService instance.
func NewRatingService(
	standingRepo ports.StandingRepository,
	ratingRepo ports.RatingHistoryRepository,
	userStatsRepo cultivationPorts.UserStatsRepository,
) ports.RatingService {
	return &ratingService{
		standingRepo:  standingRepo,
		ratingRepo:    ratingRepo,
		userStatsRepo: userStatsRepo,
	}
}

// CalculateRating computes Elo rating changes for all participants of a contest.
func (s *ratingService) CalculateRating(ctx context.Context, contestID int) error {
	log := logger.FromContext(ctx)

	// Check if ratings were already calculated for this contest.
	existing, _ := s.ratingRepo.FindByContest(ctx, contestID)
	if len(existing) > 0 {
		log.Info("rating already calculated for contest", zap.Int("contest_id", contestID))
		return nil
	}

	// Get final standings sorted by ICPC rank.
	standingsSvc := NewStandingService(s.standingRepo, nil)
	standings, err := standingsSvc.GetStandings(ctx, contestID)
	if err != nil {
		return err
	}

	if len(standings) == 0 {
		log.Info("no standings found for contest, skipping rating", zap.Int("contest_id", contestID))
		return nil
	}

	n := len(standings)

	// Gather current ratings and contest counts for each participant.
	type participant struct {
		userID       int
		currentRating int
		rank         int
		contestCount int
	}

	participants := make([]participant, n)
	for i, st := range standings {
		userID := st.UserID
		rating := 0

		// Fetch or create user stats.
		stats, err := s.userStatsRepo.GetByUserID(ctx, userID)
		if err != nil {
			// Create default user stats if not found.
			stats = &cultivationEntity.UserStats{
				UserID:           userID,
				TotalExp:         0,
				Rating:           0,
				TotalSubmissions: 0,
				AcceptedCount:    0,
			}
			if createErr := s.userStatsRepo.Create(ctx, stats); createErr != nil {
				log.Error("failed to create user stats",
					zap.Int("user_id", userID),
					zap.Error(createErr))
				rating = 0
			} else {
				rating = 0
			}
		} else {
			rating = stats.Rating
		}

		// Count how many rated contests this user has.
		count, _ := s.ratingRepo.CountByUser(ctx, userID)

		participants[i] = participant{
			userID:        userID,
			currentRating: rating,
			rank:          st.Rank,
			contestCount:  count,
		}
	}

	// Compute Elo deltas using pairwise comparison.
	deltas := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			// expected score for i against j.
			expI := 1.0 / (1.0 + math.Pow(10, float64(participants[j].currentRating-participants[i].currentRating)/eloDivisor))
			if participants[i].rank < participants[j].rank {
				// i ranked higher -> i wins.
				deltas[i] += 1.0 - expI
			} else {
				// j ranked higher -> i loses.
				deltas[i] += 0.0 - expI
			}
		}
	}

	// Apply K-factor and compute new ratings.
	records := make([]*entity.RatingHistory, n)
	statsToUpdate := make([]*cultivationEntity.UserStats, n)

	for i, p := range participants {
		k := kFactorEstablished
		if p.contestCount < minContestsForK20 {
			k = kFactorNew
		}

		newRating := p.currentRating + int(k*deltas[i])
		// Enforce rating floor of 0.
		if newRating < 0 {
			newRating = 0
		}

		records[i] = &entity.RatingHistory{
			UserID:       p.userID,
			ContestID:    contestID,
			OldRating:    p.currentRating,
			NewRating:    newRating,
			RankPosition: p.rank,
		}

		// Prepare user stats update.
		stats, err := s.userStatsRepo.GetByUserID(ctx, p.userID)
		if err != nil {
			stats = &cultivationEntity.UserStats{
				UserID: p.userID,
			}
		}
		stats.Rating = newRating
		statsToUpdate[i] = stats
	}

	// Bulk store rating history records.
	if err := s.ratingRepo.CreateBulk(ctx, records); err != nil {
		log.Error("failed to save rating history", zap.Int("contest_id", contestID), zap.Error(err))
		return err
	}

	// Update user stats with new ratings.
	for _, stats := range statsToUpdate {
		if err := s.userStatsRepo.Update(ctx, stats); err != nil {
			log.Error("failed to update user rating",
				zap.Int("user_id", stats.UserID),
				zap.Error(err))
		}
	}

	log.Info("rating calculated for contest",
		zap.Int("contest_id", contestID),
		zap.Int("participants", n))
	return nil
}

// GetContestRatingChanges returns rating changes for a contest.
func (s *ratingService) GetContestRatingChanges(ctx context.Context, contestID int) ([]*dto.RatingChangeResponse, error) {
	records, err := s.ratingRepo.FindByContest(ctx, contestID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.RatingChangeResponse, len(records))
	for i, r := range records {
		responses[i] = &dto.RatingChangeResponse{
			UserID:     r.UserID,
			Username:   "", // Will be populated via user service later.
			OldRating:  r.OldRating,
			NewRating:  r.NewRating,
			Rank:       r.RankPosition,
			Delta:      r.NewRating - r.OldRating,
		}
	}
	return responses, nil
}
