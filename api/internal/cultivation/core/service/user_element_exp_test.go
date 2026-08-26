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

func newUserElementExpTestEnv(t *testing.T) (
	*userElementExpService,
	*cultMocks.MockUserElementExpRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := cultMocks.NewMockUserElementExpRepository(ctrl)
	svc := NewUserElementExpService(repo).(*userElementExpService)
	return svc, repo
}

// --- Find ---

func TestFindUserElementExp_Success(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	records := []*entity.UserElementExp{
		{ID: 1, UserID: 10, ElementID: 5, Exp: 200},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserElementExp]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 1),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
	assert.Equal(t, int64(200), (*resp.Records)[0].Exp)
}

func TestFindUserElementExp_NilRecords(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserElementExp]{
		Records:    nil,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

func TestFindUserElementExp_RepoError(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Get ---

func TestGetUserElementExp_Success(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserElementExp{
		ID: 1, UserID: 10, ElementID: 5, Exp: 200,
	}, nil)

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, int64(200), resp.Exp)
}

func TestGetUserElementExp_NotFound(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateUserElementExp_Success(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.UserElementExp) error {
			e.ID = 1
			return nil
		},
	)

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserElementExpRequest{
		UserID:    10,
		ElementID: 5,
		Exp:       200,
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.UserID)
	assert.Equal(t, 5, resp.ElementID)
}

func TestCreateUserElementExp_RepoError(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, &cultivationDTO.CreateUserElementExpRequest{
		UserID: 10, ElementID: 5, Exp: 100,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Update ---

func TestUpdateUserElementExp_Success(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserElementExp{
		ID: 1, UserID: 10, ElementID: 5, Exp: 200,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newExp := int64(500)
	resp, err := svc.Update(ctx, 1, &cultivationDTO.UpdateUserElementExpRequest{
		Exp: &newExp,
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(500), resp.Exp)
}

func TestUpdateUserElementExp_NotFound(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Update(ctx, 999, &cultivationDTO.UpdateUserElementExpRequest{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateUserElementExp_RepoError(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.UserElementExp{
		ID: 1, UserID: 10, ElementID: 5, Exp: 200,
	}, nil)

	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	newExp := int64(500)
	resp, err := svc.Update(ctx, 1, &cultivationDTO.UpdateUserElementExpRequest{
		Exp: &newExp,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteUserElementExp_Success(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteUserElementExp_NotFound(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjUserElementExp)
}

func TestDeleteUserElementExp_ExistsError(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}

func TestDeleteUserElementExp_DeleteError(t *testing.T) {
	svc, repo := newUserElementExpTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Exists(ctx, 1).Return(true, nil)
	repo.EXPECT().Delete(ctx, 1).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}
