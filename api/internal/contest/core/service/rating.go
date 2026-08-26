package service

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/contest/ports"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	"github.com/huynhanx03/judgify/pkg/common/tx"
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
	// largeContestApproxThreshold switches to bucket approximation for expected rank.
	largeContestApproxThreshold = 2000
	// ratingBucketSize is the rating span of each approximation bucket.
	ratingBucketSize = 100
)

type ratingService struct {
	standingRepo  ports.StandingRepository
	ratingRepo    ports.RatingHistoryRepository
	userStatsRepo cultivationPorts.UserStatsRepository
	txMgr         tx.Manager
}

type ratingParticipant struct {
	userID        int
	currentRating int
	rank          int
	actualRank    float64
	solvedCount   int
	penalty       int
	contestCount  int
	stats         *cultivationEntity.UserStats
}

type ratingBucket struct {
	representativeRating int
	count                int
}

// NewRatingService creates a new RatingService instance.
func NewRatingService(
	standingRepo ports.StandingRepository,
	ratingRepo ports.RatingHistoryRepository,
	userStatsRepo cultivationPorts.UserStatsRepository,
	txMgr tx.Manager,
) ports.RatingService {
	return &ratingService{
		standingRepo:  standingRepo,
		ratingRepo:    ratingRepo,
		userStatsRepo: userStatsRepo,
		txMgr:         txMgr,
	}
}

// CalculateRating computes expected-rank rating changes for all participants of a contest.
func (s *ratingService) CalculateRating(ctx context.Context, contestID int) error {
	if s.txMgr == nil {
		return errors.New("transaction manager is required")
	}

	log := logger.FromContext(ctx)

	return s.txMgr.DoInTx(ctx, func(txCtx context.Context) error {
		// Check if ratings were already calculated for this contest.
		existing, err := s.ratingRepo.FindByContest(txCtx, contestID)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			log.Info("rating already calculated for contest", zap.Int("contest_id", contestID))
			return nil
		}

		// Get final standings sorted by ICPC rank.
		standingsSvc := NewStandingService(s.standingRepo, nil)
		standings, err := standingsSvc.GetStandings(txCtx, contestID)
		if err != nil {
			return err
		}

		if len(standings) == 0 {
			log.Info("no standings found for contest, skipping rating", zap.Int("contest_id", contestID))
			return nil
		}

		n := len(standings)
		userIDs := standingUserIDs(standings)

		statsByUserID, err := s.userStatsRepo.GetByUserIDs(txCtx, userIDs)
		if err != nil {
			return err
		}

		contestCounts, err := s.ratingRepo.CountByUsers(txCtx, userIDs)
		if err != nil {
			return err
		}

		// Gather current ratings and contest counts for each participant.
		participants := make([]ratingParticipant, n)
		for i, st := range standings {
			stats := statsByUserID[st.UserID]
			if stats == nil {
				stats = &cultivationEntity.UserStats{
					UserID:           st.UserID,
					TotalExp:         0,
					Rating:           0,
					TotalSubmissions: 0,
					AcceptedCount:    0,
				}
				if err := s.userStatsRepo.Create(txCtx, stats); err != nil {
					return err
				}
				statsByUserID[st.UserID] = stats
			}

			participants[i] = ratingParticipant{
				userID:        st.UserID,
				currentRating: stats.Rating,
				rank:          st.Rank,
				solvedCount:   st.SolvedCount,
				penalty:       st.Penalty,
				contestCount:  contestCounts[st.UserID],
				stats:         stats,
			}
		}

		assignActualRanks(participants)
		expectedRanks := calculateExpectedRanks(participants)

		// Apply K-factor and compute new ratings.
		records := make([]*entity.RatingHistory, n)
		ratingsByUserID := make(map[int]int, n)
		for i, p := range participants {
			k := kFactorEstablished
			if p.contestCount < minContestsForK20 {
				k = kFactorNew
			}

			newRating := p.currentRating + ratingDelta(expectedRanks[i], p.actualRank, k, n)
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

			participants[i].stats.Rating = newRating
			ratingsByUserID[p.userID] = newRating
		}

		// Bulk store rating history records.
		if err := s.ratingRepo.CreateBulk(txCtx, records); err != nil {
			log.Error("failed to save rating history", zap.Int("contest_id", contestID), zap.Error(err))
			return err
		}

		// Update all user ratings in one bulk statement.
		if err := s.userStatsRepo.UpdateRatings(txCtx, ratingsByUserID); err != nil {
			log.Error("failed to bulk update user ratings", zap.Error(err))
			return err
		}

		log.Info("rating calculated for contest",
			zap.Int("contest_id", contestID),
			zap.Int("participants", n))
		return nil
	})
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
			UserID:    r.UserID,
			Username:  r.Username,
			OldRating: r.OldRating,
			NewRating: r.NewRating,
			Rank:      r.RankPosition,
			Delta:     r.NewRating - r.OldRating,
		}
	}
	return responses, nil
}

func standingUserIDs(standings []*dto.StandingResponse) []int {
	userIDs := make([]int, 0, len(standings))
	seen := make(map[int]struct{}, len(standings))
	for _, st := range standings {
		if _, ok := seen[st.UserID]; ok {
			continue
		}
		seen[st.UserID] = struct{}{}
		userIDs = append(userIDs, st.UserID)
	}
	return userIDs
}

func assignActualRanks(participants []ratingParticipant) {
	for start := 0; start < len(participants); {
		end := start
		for end+1 < len(participants) && sameContestResult(participants[start], participants[end+1]) {
			end++
		}

		actualRank := (float64(start+1) + float64(end+1)) / 2.0
		for i := start; i <= end; i++ {
			participants[i].rank = start + 1
			participants[i].actualRank = actualRank
		}

		start = end + 1
	}
}

func sameContestResult(a, b ratingParticipant) bool {
	return a.solvedCount == b.solvedCount && a.penalty == b.penalty
}

func calculateExpectedRanks(participants []ratingParticipant) []float64 {
	if shouldUseApproximation(len(participants)) {
		return calculateExpectedRanksByBuckets(participants)
	}
	return calculateExpectedRanksExact(participants)
}

func shouldUseApproximation(participantCount int) bool {
	return participantCount > largeContestApproxThreshold
}

func calculateExpectedRanksExact(participants []ratingParticipant) []float64 {
	expectedRanks := make([]float64, len(participants))
	for i := range expectedRanks {
		expectedRanks[i] = 1.0
	}

	scoreCache := make(map[int]float64, len(participants))
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			diff := participants[j].currentRating - participants[i].currentRating
			iBeatsJ := expectedScoreByDiff(diff, scoreCache)
			expectedRanks[i] += 1.0 - iBeatsJ
			expectedRanks[j] += iBeatsJ
		}
	}

	return expectedRanks
}

func calculateExpectedRanksByBuckets(participants []ratingParticipant) []float64 {
	buckets := buildRatingBuckets(participants)
	expectedRanks := make([]float64, len(participants))
	scoreCache := make(map[int]float64, len(buckets)*4)

	for i, p := range participants {
		seed := 1.0
		for _, bucket := range buckets {
			diff := p.currentRating - bucket.representativeRating
			probabilityBucketBeatsParticipant := expectedScoreByDiff(diff, scoreCache)
			seed += float64(bucket.count) * probabilityBucketBeatsParticipant
		}
		// Exclude self-match from bucket aggregation.
		seed -= 0.5
		expectedRanks[i] = seed
	}
	return expectedRanks
}

func buildRatingBuckets(participants []ratingParticipant) []ratingBucket {
	bucketCounts := make(map[int]int)
	bucketRatingSums := make(map[int]int)

	for _, p := range participants {
		bucketID := p.currentRating / ratingBucketSize
		bucketCounts[bucketID]++
		bucketRatingSums[bucketID] += p.currentRating
	}

	bucketIDs := make([]int, 0, len(bucketCounts))
	for bucketID := range bucketCounts {
		bucketIDs = append(bucketIDs, bucketID)
	}
	// Deterministic order keeps calculations stable across runs.
	sort.Ints(bucketIDs)

	buckets := make([]ratingBucket, 0, len(bucketIDs))
	for _, bucketID := range bucketIDs {
		count := bucketCounts[bucketID]
		representativeRating := bucketRatingSums[bucketID] / count
		buckets = append(buckets, ratingBucket{
			representativeRating: representativeRating,
			count:                count,
		})
	}
	return buckets
}

func expectedScoreByDiff(diff int, cache map[int]float64) float64 {
	if score, ok := cache[diff]; ok {
		return score
	}
	score := 1.0 / (1.0 + math.Pow(10, float64(diff)/eloDivisor))
	cache[diff] = score
	return score
}

func ratingDelta(expectedRank, actualRank, k float64, participantCount int) int {
	if participantCount <= 1 {
		return 0
	}

	performance := (expectedRank - actualRank) / float64(participantCount-1)
	return int(math.Round(k * performance))
}
