package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	contestMocks "github.com/huynhanx03/judgify/internal/contest/mocks"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newRatingTestEnv(t *testing.T) (
	*ratingService,
	*contestMocks.MockStandingRepository,
	*contestMocks.MockRatingHistoryRepository,
	*cultMocks.MockUserStatsRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	standingRepo := contestMocks.NewMockStandingRepository(ctrl)
	ratingRepo := contestMocks.NewMockRatingHistoryRepository(ctrl)
	userStatsRepo := cultMocks.NewMockUserStatsRepository(ctrl)
	svc := NewRatingService(standingRepo, ratingRepo, userStatsRepo).(*ratingService)
	return svc, standingRepo, ratingRepo, userStatsRepo
}

func TestGetContestRatingChanges_Success(t *testing.T) {
	svc, _, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{
		{UserID: 10, OldRating: 1000, NewRating: 1020, RankPosition: 1},
		{UserID: 20, OldRating: 1500, NewRating: 1480, RankPosition: 2},
	}, nil)

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, 20, resp[0].Delta)  // 1020 - 1000
	assert.Equal(t, -20, resp[1].Delta) // 1480 - 1500
}

func TestGetContestRatingChanges_Empty(t *testing.T) {
	svc, _, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{}, nil)

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestGetContestRatingChanges_RepoError(t *testing.T) {
	svc, _, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetContestRatingChanges(ctx, 1)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCalculateRating_AlreadyCalculated(t *testing.T) {
	svc, _, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.RatingHistory{{UserID: 10}}, nil)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
}

func TestCalculateRating_EmptyStandings(t *testing.T) {
	svc, standingRepo, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	// StandingService.GetStandings calls FindByContest
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{}, nil)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
}

func TestCalculateRating_TwoParticipants(t *testing.T) {
	svc, standingRepo, ratingRepo, userStatsRepo := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)

	// Standings: user 10 rank 1 (higher), user 20 rank 2
	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 5, Penalty: 100},
		{ContestID: 1, UserID: 20, SolvedCount: 3, Penalty: 200},
	}, nil)

	// First pass: fetch user stats for each participant
	userStatsRepo.EXPECT().GetByUserID(ctx, 10).Return(&cultivationEntity.UserStats{UserID: 10, Rating: 1000}, nil)
	userStatsRepo.EXPECT().GetByUserID(ctx, 20).Return(&cultivationEntity.UserStats{UserID: 20, Rating: 1500}, nil)

	// CountByUser for contest count
	ratingRepo.EXPECT().CountByUser(ctx, 10).Return(0, nil) // new user → K=40
	ratingRepo.EXPECT().CountByUser(ctx, 20).Return(0, nil)

	// Bulk create rating history
	ratingRepo.EXPECT().CreateBulk(gomock.Any(), gomock.Any()).Return(nil)

	// Second pass: update user stats
	userStatsRepo.EXPECT().GetByUserID(ctx, 10).Return(&cultivationEntity.UserStats{ID: 1, UserID: 10, Rating: 1000}, nil)
	userStatsRepo.EXPECT().GetByUserID(ctx, 20).Return(&cultivationEntity.UserStats{ID: 2, UserID: 20, Rating: 1500}, nil)
	userStatsRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	userStatsRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := svc.CalculateRating(ctx, 1)
	assert.NoError(t, err)
}

func TestCalculateRating_StandingRepoError(t *testing.T) {
	svc, standingRepo, ratingRepo, _ := newRatingTestEnv(t)
	ctx := context.Background()

	ratingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, nil)
	standingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	err := svc.CalculateRating(ctx, 1)
	assert.Error(t, err)
}
