package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	cultivationDTO "github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newUserStatsTestEnv(t *testing.T) (
	*userStatsService,
	*cultMocks.MockUserStatsRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := cultMocks.NewMockUserStatsRepository(ctrl)
	svc := NewUserStatsService(repo).(*userStatsService)
	return svc, repo
}

// --- Find ---

func TestFindUserStats_Success(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	records := []*entity.UserStats{
		{ID: 1, UserID: 10, TotalExp: 500, Rating: 1200},
		{ID: 2, UserID: 20, TotalExp: 1000, Rating: 1500},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserStats]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindUserStats_NilRecords(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserStats]{
		Records:    nil,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

func TestFindUserStats_RepoError(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Get ---

func TestGetUserStats_Success(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserStats{
		ID: 1, UserID: 10, TotalExp: 500, Rating: 1200,
	}, nil)

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.UserID)
}

func TestGetUserStats_NotFound(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateUserStats_Success(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.UserStats) error {
			e.ID = 1
			return nil
		},
	)

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserStatsRequest{
		UserID:   10,
		TotalExp: 500,
		Rating:   1200,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.UserID)
}

func TestCreateUserStats_RepoError(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserStatsRequest{
		UserID: 10,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Update ---

func TestUpdateUserStats_Success(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserStats{
		ID: 1, UserID: 10, TotalExp: 500, Rating: 1200,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newExp := int64(1000)
	newRating := 1500
	resp, err := svc.Update(ctx, 1, &cultivationDTO.UpdateUserStatsRequest{
		TotalExp: &newExp,
		Rating:   &newRating,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1000), resp.TotalExp)
	assert.Equal(t, 1500, resp.Rating)
}

func TestUpdateUserStats_NotFound(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Update(ctx, 999, &cultivationDTO.UpdateUserStatsRequest{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateUserStats_RepoError(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserStats{
		ID: 1, UserID: 10, TotalExp: 500, Rating: 1200,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	newExp := int64(1000)
	resp, err := svc.Update(ctx, 1, &cultivationDTO.UpdateUserStatsRequest{
		TotalExp: &newExp,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteUserStats_Success(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteUserStats_NotFound(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjUserStats)
}

func TestDeleteUserStats_ExistsError(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}

func TestDeleteUserStats_DeleteError(t *testing.T) {
	svc, repo := newUserStatsTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}
