package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newRankingTestEnv(t *testing.T) (*rankingService, *cultMocks.MockUserStatsRepository, *cultMocks.MockRankRepository, *cultMocks.MockLevelRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	userStatsRepo := cultMocks.NewMockUserStatsRepository(ctrl)
	rankRepo := cultMocks.NewMockRankRepository(ctrl)
	levelRepo := cultMocks.NewMockLevelRepository(ctrl)
	c := ember.New[string, any]()
	svc := NewRankingService(userStatsRepo, rankRepo, levelRepo, c).(*rankingService)
	return svc, userStatsRepo, rankRepo, levelRepo
}

func TestGetTopByRating_Success(t *testing.T) {
	svc, userStatsRepo, rankRepo, _ := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "rating", 10).Return([]*entity.UserStatsWithUser{
		{Stats: &entity.UserStats{UserID: 1, Rating: 1500}, Username: "alice"},
		{Stats: &entity.UserStats{UserID: 2, Rating: 1200}, Username: "bob"},
	}, nil)

	rankRepo.EXPECT().FindAll(ctx).Return([]*entity.Rank{
		{ID: 1, Name: "Gold", MinRating: 1400},
		{ID: 2, Name: "Silver", MinRating: 1200},
		{ID: 3, Name: "Bronze", MinRating: 0},
	}, nil)

	resp, err := svc.GetTopByRating(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Gold", resp[0].RankTitle)
	assert.Equal(t, "Silver", resp[1].RankTitle)
}

func TestGetTopByRating_RepoError(t *testing.T) {
	svc, userStatsRepo, _, _ := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "rating", 10).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetTopByRating(ctx, 10)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTopByRating_RankRepoError(t *testing.T) {
	svc, userStatsRepo, rankRepo, _ := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "rating", 10).Return([]*entity.UserStatsWithUser{
		{Stats: &entity.UserStats{UserID: 1, Rating: 1500}, Username: "alice"},
	}, nil)

	rankRepo.EXPECT().FindAll(ctx).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetTopByRating(ctx, 10)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTopByExp_Success(t *testing.T) {
	svc, userStatsRepo, _, levelRepo := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "total_exp", 10).Return([]*entity.UserStatsWithUser{
		{Stats: &entity.UserStats{UserID: 1, TotalExp: 5000}, Username: "alice"},
	}, nil)

	levelRepo.EXPECT().FindAll(ctx).Return([]*entity.Level{
		{ID: 1, Name: "Master", MinExp: 5000},
		{ID: 2, Name: "Expert", MinExp: 2000},
		{ID: 3, Name: "Novice", MinExp: 0},
	}, nil)

	resp, err := svc.GetTopByExp(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	// sortedAsc = [Novice:0, Expert:2000, Master:5000]
	// First match where MinExp <= 5000 is Novice(0)
	assert.Equal(t, "Novice", resp[0].LevelName)
}

func TestGetTopByExp_RepoError(t *testing.T) {
	svc, userStatsRepo, _, _ := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "total_exp", 10).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetTopByExp(ctx, 10)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTopByExp_LevelRepoError(t *testing.T) {
	svc, userStatsRepo, _, levelRepo := newRankingTestEnv(t)
	ctx := context.Background()

	userStatsRepo.EXPECT().GetTopSortedWithUser(ctx, "total_exp", 10).Return([]*entity.UserStatsWithUser{
		{Stats: &entity.UserStats{UserID: 1, TotalExp: 5000}, Username: "alice"},
	}, nil)

	levelRepo.EXPECT().FindAll(ctx).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))

	resp, err := svc.GetTopByExp(ctx, 10)
	assert.Nil(t, resp)
	assert.Error(t, err)
}
