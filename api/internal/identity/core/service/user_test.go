package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	idMocks "github.com/huynhanx03/judgify/internal/identity/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	txMocks "github.com/huynhanx03/judgify/pkg/common/tx/mocks"
)

func newUserTestEnv(t *testing.T) (
	*userService,
	*idMocks.MockUserRepository,
	*idMocks.MockCredentialRepository,
	*idMocks.MockRoleRepository,
	*idMocks.MockAttributeDefinitionRepository,
	*idMocks.MockUserAttributeValueRepository,
	*txMocks.MockManager,
	cache.LocalCache[string, any],
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	userRepo := idMocks.NewMockUserRepository(ctrl)
	credRepo := idMocks.NewMockCredentialRepository(ctrl)
	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	attrDefRepo := idMocks.NewMockAttributeDefinitionRepository(ctrl)
	attrValRepo := idMocks.NewMockUserAttributeValueRepository(ctrl)
	txMgr := txMocks.NewMockManager(ctrl)

	localCache := ember.New[string, any]()

	svc := NewUserService(
		userRepo, credRepo, roleRepo, attrDefRepo, attrValRepo,
		localCache, txMgr,
	).(*userService)

	return svc, userRepo, credRepo, roleRepo, attrDefRepo, attrValRepo, txMgr, localCache
}

// --- Delete ---

func TestDeleteUser_Success(t *testing.T) {
	svc, userRepo, _, _, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Exists(ctx, 1).Return(true, nil)
	userRepo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteUser_NotFound(t *testing.T) {
	svc, userRepo, _, _, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjUser)
}

// --- UpdateUser ---

func TestUpdateUser_Success(t *testing.T) {
	svc, userRepo, _, roleRepo, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Get(ctx, 1).Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   1,
	}, nil)

	roleRepo.EXPECT().Get(ctx, 2).Return(&entity.Role{
		ID:   2,
		Name: "admin",
	}, nil)

	userRepo.EXPECT().Update(ctx, gomock.Any()).Return(nil)

	resp, err := svc.UpdateUser(ctx, &dto.UpdateUserRequest{
		ID:     1,
		RoleID: 2,
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, resp.RoleID)
	assert.Equal(t, "admin", resp.RoleName)
}

func TestUpdateUser_UserNotFound(t *testing.T) {
	svc, userRepo, _, _, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "user not found", nil))

	resp, err := svc.UpdateUser(ctx, &dto.UpdateUserRequest{
		ID:     999,
		RoleID: 2,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- GetRole ---

func TestGetUserRole_Success(t *testing.T) {
	svc, userRepo, _, roleRepo, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Get(ctx, 1).Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   2,
	}, nil)

	roleRepo.EXPECT().Get(ctx, 2).Return(&entity.Role{
		ID:    2,
		Name:  "admin",
		Level: 0,
	}, nil)

	resp, err := svc.GetRole(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, "admin", resp.Name)
	assert.Equal(t, 2, resp.ID)
}

// --- GetProfile ---

func TestGetProfile_UserNotFound(t *testing.T) {
	svc, userRepo, _, _, _, _, _, _ := newUserTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "user not found", nil))

	resp, err := svc.GetProfile(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}
