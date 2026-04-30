package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	contestMocks "github.com/huynhanx03/judgify/internal/contest/mocks"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	txMocks "github.com/huynhanx03/judgify/pkg/common/tx/mocks"
)

func newRatingTestEnv(t *testing.T) (
	*ratingService,
	*contestMocks.MockStandingRepository,
	*contestMocks.MockRatingHistoryRepository,
	*cultMocks.MockUserStatsRepository,
	*txMocks.MockManager,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	standingRepo := contestMocks.NewMockStandingRepository(ctrl)
	ratingRepo := contestMocks.NewMockRatingHistoryRepository(ctrl)
	userStatsRepo := cultMocks.NewMockUserStatsRepository(ctrl)
	txMgr := txMocks.NewMockManager(ctrl)
	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	).AnyTimes()
	svc := NewRatingService(standingRepo, ratingRepo, userStatsRepo, txMgr).(*ratingService)
	return svc, standingRepo, ratingRepo, userStatsRepo, txMgr
}

func TestGetContestRatingChanges_Success(t *testing.T) {
	svc, _, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{
		{UserID: 10, Username: "alice", OldRating: 1000, NewRating: 1020, RankPosition: 1},
		{UserID: 20, Username: "bob", OldRating: 1500, NewRating: 1480, RankPosition: 2},
	}, nil)

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "alice", resp[0].Username)
	assert.Equal(t, "bob", resp[1].Username)
	assert.Equal(t, 20, resp[0].Delta)  // 1020 - 1000
	assert.Equal(t, -20, resp[1].Delta) // 1480 - 1500
}

func TestGetContestRatingChanges_Empty(t *testing.T) {
	svc, _, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{}, nil)

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestGetContestRatingChanges_RepoError(t *testing.T) {
	svc, _, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCalculateRating_AlreadyCalculated(t *testing.T) {
	svc, _, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{{UserID: 10}}, nil)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
}

func TestCalculateRating_EmptyStandings(t *testing.T) {
	svc, standingRepo, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	// StandingService.GetStandings calls FindByContest
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{}, nil)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
}

func TestCalculateRating_TwoParticipants(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)

	// Standings: user 10 rank 1 (higher), user 20 rank 2
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 5, Penalty: 100},
		{ContestID: 1, UserID: 20, SolvedCount: 3, Penalty: 200},
	}, nil)

	userStatsRepo.EXPECT().GetByUserIDs(ctx, []int{10, 20}).Return(map[int]*cultivationEntity.UserStats{
		10: {ID: 1, UserID: 10, Rating: 1000},
		20: {ID: 2, UserID: 20, Rating: 1500},
	}, nil)
	ratingRepo.EXPECT().CountByUsers(ctx, []int{10, 20}).Return(map[int]int{
		10: 0, // new user -> K=40
		20: 0,
	}, nil)

	var gotRecords []*entity.RatingHistory
	ratingRepo.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, records []*entity.RatingHistory) error {
			gotRecords = append([]*entity.RatingHistory(nil), records...)
			return nil
		},
	)

	expectedRatings := map[int]int{
		10: 1038, // 1000 + round(40*((1.946... - 1)/(2-1)))
		20: 1462, // 1500 + round(40*((1.053... - 2)/(2-1)))
	}
	userStatsRepo.EXPECT().UpdateRatings(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ratingsByUserID map[int]int) error {
			assert.Equal(t, expectedRatings, ratingsByUserID)
			return nil
		},
	)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, gotRecords, 2)

	byUser := map[int]*entity.RatingHistory{}
	for _, rec := range gotRecords {
		byUser[rec.UserID] = rec
	}
	assert.Equal(t, 1000, byUser[10].OldRating)
	assert.Equal(t, 1038, byUser[10].NewRating)
	assert.Equal(t, 1, byUser[10].RankPosition)
	assert.Equal(t, 1500, byUser[20].OldRating)
	assert.Equal(t, 1462, byUser[20].NewRating)
	assert.Equal(t, 2, byUser[20].RankPosition)
}

func TestCalculateRating_NormalizesDeltaByFieldSize(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 3, Penalty: 100},
		{ContestID: 1, UserID: 20, SolvedCount: 2, Penalty: 200},
		{ContestID: 1, UserID: 30, SolvedCount: 1, Penalty: 300},
	}, nil)

	userStatsRepo.EXPECT().GetByUserIDs(ctx, []int{10, 20, 30}).Return(map[int]*cultivationEntity.UserStats{
		10: {ID: 1, UserID: 10, Rating: 1200},
		20: {ID: 2, UserID: 20, Rating: 1200},
		30: {ID: 3, UserID: 30, Rating: 1200},
	}, nil)
	ratingRepo.EXPECT().CountByUsers(ctx, []int{10, 20, 30}).Return(map[int]int{
		10: 0,
		20: 0,
		30: 0,
	}, nil)

	var gotRecords []*entity.RatingHistory
	ratingRepo.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, records []*entity.RatingHistory) error {
			gotRecords = append([]*entity.RatingHistory(nil), records...)
			return nil
		},
	)
	expectedRatings := map[int]int{
		10: 1220,
		20: 1200,
		30: 1180,
	}
	userStatsRepo.EXPECT().UpdateRatings(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ratingsByUserID map[int]int) error {
			assert.Equal(t, expectedRatings, ratingsByUserID)
			return nil
		},
	)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, gotRecords, 3)

	byUser := map[int]*entity.RatingHistory{}
	for _, rec := range gotRecords {
		byUser[rec.UserID] = rec
	}
	assert.Equal(t, 1220, byUser[10].NewRating)
	assert.Equal(t, 1200, byUser[20].NewRating)
	assert.Equal(t, 1180, byUser[30].NewRating)
}

func TestCalculateRating_TieGivesDraw(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	// Equal solved_count and penalty => draw.
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 5, Penalty: 100},
		{ContestID: 1, UserID: 20, SolvedCount: 5, Penalty: 100},
	}, nil)

	userStatsRepo.EXPECT().GetByUserIDs(ctx, []int{10, 20}).Return(map[int]*cultivationEntity.UserStats{
		10: {ID: 1, UserID: 10, Rating: 1200},
		20: {ID: 2, UserID: 20, Rating: 1200},
	}, nil)
	ratingRepo.EXPECT().CountByUsers(ctx, []int{10, 20}).Return(map[int]int{
		10: 10, // established users => K=20
		20: 10,
	}, nil)

	var gotRecords []*entity.RatingHistory
	ratingRepo.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, records []*entity.RatingHistory) error {
			gotRecords = append([]*entity.RatingHistory(nil), records...)
			return nil
		},
	)
	userStatsRepo.EXPECT().UpdateRatings(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, ratingsByUserID map[int]int) error {
			assert.Equal(t, map[int]int{10: 1200, 20: 1200}, ratingsByUserID)
			return nil
		},
	)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, gotRecords, 2)
	for _, rec := range gotRecords {
		assert.Equal(t, rec.OldRating, rec.NewRating)
		assert.Equal(t, 1, rec.RankPosition)
	}
}

func TestCalculateRating_ExistingHistoryError(t *testing.T) {
	svc, _, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	err := svc.CalculateRating(ctx, 1)
	assert.Error(t, err)
}

func TestCalculateRating_UserStatsLookupError(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 1, Penalty: 100},
	}, nil)
	userStatsRepo.EXPECT().GetByUserIDs(ctx, []int{10}).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	err := svc.CalculateRating(ctx, 1)
	assert.Error(t, err)
}

func TestCalculateRating_CountByUserError(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 1, Penalty: 100},
	}, nil)
	userStatsRepo.EXPECT().GetByUserIDs(ctx, []int{10}).Return(map[int]*cultivationEntity.UserStats{
		10: {ID: 1, UserID: 10, Rating: 1200},
	}, nil)
	ratingRepo.EXPECT().CountByUsers(ctx, []int{10}).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	err := svc.CalculateRating(ctx, 1)
	assert.Error(t, err)
}

func TestCalculateRating_StandingRepoError(t *testing.T) {
	svc, standingRepo, ratingRepo, _, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	standingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	err := svc.CalculateRating(ctx, 1)
	assert.Error(t, err)
}

func TestExpectedScoreByDiff_CacheHit(t *testing.T) {
	cache := map[int]float64{}
	first := expectedScoreByDiff(120, cache)
	second := expectedScoreByDiff(120, cache)

	assert.Equal(t, first, second)
	assert.Len(t, cache, 1)
}

func TestCalculateExpectedRanksByBuckets_AllSameRating(t *testing.T) {
	participants := make([]ratingParticipant, 5000)
	for i := range participants {
		participants[i].currentRating = 1500
	}

	expectedRanks := calculateExpectedRanksByBuckets(participants)
	expectedSeed := float64(len(participants)+1) / 2.0

	assert.Len(t, expectedRanks, len(participants))
	for _, seed := range expectedRanks {
		assert.InDelta(t, expectedSeed, seed, 1e-9)
	}
}

func TestShouldUseApproximation(t *testing.T) {
	assert.False(t, shouldUseApproximation(largeContestApproxThreshold))
	assert.True(t, shouldUseApproximation(largeContestApproxThreshold+1))
}
