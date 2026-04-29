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

func newUserTraitTestEnv(t *testing.T) (
	*userTraitService,
	*cultMocks.MockUserTraitRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := cultMocks.NewMockUserTraitRepository(ctrl)
	svc := NewUserTraitService(repo).(*userTraitService)
	return svc, repo
}

// --- Find ---

func TestFindUserTraits_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	records := []*entity.UserTrait{
		{ID: 1, UserID: 10, TraitID: 100},
		{ID: 2, UserID: 10, TraitID: 200},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserTrait]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindUserTraits_NilRecords(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserTrait]{
		Records:    nil,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

func TestFindUserTraits_RepoError(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Get ---

func TestGetUserTrait_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserTrait{
		ID: 1, UserID: 10, TraitID: 100,
	}, nil)

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.UserID)
	assert.Equal(t, 100, resp.TraitID)
}

func TestGetUserTrait_NotFound(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateUserTrait_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.UserTrait) error {
			e.ID = 1
			return nil
		},
	)

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserTraitRequest{
		UserID:  10,
		TraitID: 100,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.UserID)
	assert.Equal(t, 100, resp.TraitID)
}

func TestCreateUserTrait_RepoError(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserTraitRequest{
		UserID: 10, TraitID: 100,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteUserTrait_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteUserTrait_NotFound(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjUserTrait)
}

func TestDeleteUserTrait_ExistsError(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}

func TestDeleteUserTrait_DeleteError(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}
