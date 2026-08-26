package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	identityDTO "github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	identityMocks "github.com/huynhanx03/judgify/internal/identity/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newResourceTestEnv(t *testing.T) (*resourceService, *identityMocks.MockResourceRepository, *identityMocks.MockCacheService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockResourceRepository(ctrl)
	cacheSvc := identityMocks.NewMockCacheService(ctrl)
	svc := NewResourceService(repo, cacheSvc).(*resourceService)
	return svc, repo, cacheSvc
}

func TestFindAllResources_Success(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Resource{{ID: 1, Key: "users"}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindAllResources_RepoError(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindResources_Success(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	records := []*entity.Resource{{ID: 1, Key: "users"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Resource]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindResources_NilRecords(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Resource]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestGetResource_Success(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Resource{ID: 1, Key: "users"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "users", resp.Key)
}

func TestGetResource_NotFound(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateResource_Success(t *testing.T) {
	svc, repo, cacheSvc := newResourceTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Resource) error { e.ID = 1; return nil })
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	resp, err := svc.Create(context.Background(), &identityDTO.CreateResourceRequest{Key: "users"})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateResource_RepoError(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &identityDTO.CreateResourceRequest{Key: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateResource_Success(t *testing.T) {
	svc, repo, cacheSvc := newResourceTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Resource{ID: 1, Key: "users"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	newKey := "accounts"
	resp, err := svc.Update(context.Background(), 1, &identityDTO.UpdateResourceRequest{Key: &newKey})
	assert.NoError(t, err)
	assert.Equal(t, "accounts", resp.Key)
}

func TestUpdateResource_NotFound(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &identityDTO.UpdateResourceRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteResource_Success(t *testing.T) {
	svc, repo, cacheSvc := newResourceTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteResource_NotFound(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjResource)
}

func TestDeleteResource_ExistsError(t *testing.T) {
	svc, repo, _ := newResourceTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}
