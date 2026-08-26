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

func newPermissionTestEnv(t *testing.T) (*permissionService, *identityMocks.MockPermissionRepository, *identityMocks.MockCacheService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockPermissionRepository(ctrl)
	cacheSvc := identityMocks.NewMockCacheService(ctrl)
	svc := NewPermissionService(repo, cacheSvc).(*permissionService)
	return svc, repo, cacheSvc
}

func TestFindAllPermissions_Success(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Permission{{ID: 1}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindAllPermissions_RepoError(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindPermissions_Success(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	records := []*entity.Permission{{ID: 1}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Permission]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindPermissions_NilRecords(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Permission]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestGetPermission_Success(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Permission{ID: 1, RoleID: 2, ResourceID: 3}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestGetPermission_NotFound(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreatePermission_Success(t *testing.T) {
	svc, repo, cacheSvc := newPermissionTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Permission) error { e.ID = 1; return nil })
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	resp, err := svc.Create(context.Background(), &identityDTO.CreatePermissionRequest{RoleID: 2, ResourceID: 3})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreatePermission_RepoError(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &identityDTO.CreatePermissionRequest{RoleID: 1})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdatePermission_Success(t *testing.T) {
	svc, repo, cacheSvc := newPermissionTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Permission{ID: 1, Scopes: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	newScopes := 7
	resp, err := svc.Update(context.Background(), 1, &identityDTO.UpdatePermissionRequest{Scopes: &newScopes})
	assert.NoError(t, err)
	assert.Equal(t, 7, resp.Scopes)
}

func TestUpdatePermission_NotFound(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &identityDTO.UpdatePermissionRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeletePermission_Success(t *testing.T) {
	svc, repo, cacheSvc := newPermissionTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeletePermission_NotFound(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjPermission)
}

func TestDeletePermission_ExistsError(t *testing.T) {
	svc, repo, _ := newPermissionTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}
