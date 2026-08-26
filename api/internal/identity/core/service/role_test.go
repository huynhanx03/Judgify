package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	idMocks "github.com/huynhanx03/judgify/internal/identity/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	txMocks "github.com/huynhanx03/judgify/pkg/common/tx/mocks"
)

func newRoleTestEnv(t *testing.T) (
	*roleService,
	*idMocks.MockRoleRepository,
	*idMocks.MockCacheService,
	*txMocks.MockManager,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	cacheSvc := idMocks.NewMockCacheService(ctrl)
	txMgr := txMocks.NewMockManager(ctrl)

	svc := NewRoleService(roleRepo, cacheSvc, txMgr).(*roleService)
	return svc, roleRepo, cacheSvc, txMgr
}

// --- FindAll ---

func TestFindAllRoles_Success(t *testing.T) {
	svc, roleRepo, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().FindAll(ctx).Return([]*entity.Role{
		{ID: 1, Name: "admin", Level: 0, ParentID: -1},
		{ID: 2, Name: "student", Level: 1, ParentID: 1},
	}, nil)

	roles, err := svc.FindAll(ctx)

	assert.NoError(t, err)
	assert.Len(t, roles, 2)
	assert.Equal(t, "admin", roles[0].Name)
	assert.Equal(t, "student", roles[1].Name)
}

func TestFindAllRoles_RepoError(t *testing.T) {
	svc, roleRepo, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().FindAll(ctx).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	roles, err := svc.FindAll(ctx)

	assert.Nil(t, roles)
	assert.Error(t, err)
}

// --- Get ---

func TestGetRole_Success(t *testing.T) {
	svc, roleRepo, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().Get(ctx, 1).Return(&entity.Role{
		ID:   1,
		Name: "admin",
	}, nil)

	role, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, "admin", role.Name)
	assert.Equal(t, 1, role.ID)
}

func TestGetRole_NotFound(t *testing.T) {
	svc, roleRepo, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "role not found", nil))

	role, err := svc.Get(ctx, 999)

	assert.Nil(t, role)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateRole_Success(t *testing.T) {
	svc, roleRepo, cacheSvc, txMgr := newRoleTestEnv(t)
	ctx := context.Background()

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	roleRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *entity.Role) error {
			r.ID = 3
			return nil
		},
	)

	// rebuildTree calls FindAll then UpdateBulk
	roleRepo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Role{
		{ID: 1, Name: "admin", ParentID: -1},
		{ID: 3, Name: "moderator", ParentID: 1},
	}, nil)
	roleRepo.EXPECT().UpdateBulk(gomock.Any(), gomock.Any()).Return(nil)

	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)

	parentID := 1
	role, err := svc.Create(ctx, &dto.CreateRoleRequest{
		Name:     "moderator",
		Level:    1,
		ParentID: &parentID,
	})

	assert.NoError(t, err)
	assert.Equal(t, "moderator", role.Name)
}

// --- Update ---

func TestUpdateRole_ParentChange(t *testing.T) {
	svc, roleRepo, cacheSvc, txMgr := newRoleTestEnv(t)
	ctx := context.Background()

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	roleRepo.EXPECT().Get(ctx, 2).Return(&entity.Role{
		ID:       2,
		Name:     "student",
		Level:    2,
		ParentID: 1,
	}, nil)

	roleRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	// rebuildTree
	roleRepo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Role{
		{ID: 1, Name: "admin", ParentID: -1},
		{ID: 2, Name: "student", ParentID: 3},
	}, nil)
	roleRepo.EXPECT().UpdateBulk(gomock.Any(), gomock.Any()).Return(nil)

	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)

	newParent := 3
	role, err := svc.Update(ctx, 2, &dto.UpdateRoleRequest{
		ParentID: &newParent,
	})

	assert.NoError(t, err)
	assert.NotNil(t, role)
}

func TestUpdateRole_SelfParent(t *testing.T) {
	svc, roleRepo, _, txMgr := newRoleTestEnv(t)
	ctx := context.Background()

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	roleRepo.EXPECT().Get(ctx, 2).Return(&entity.Role{
		ID:       2,
		Name:     "student",
		ParentID: 1,
	}, nil)

	selfID := 2
	role, err := svc.Update(ctx, 2, &dto.UpdateRoleRequest{
		ParentID: &selfID,
	})

	assert.Nil(t, role)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	require.True(t, ok)
	assert.Equal(t, response.CodeInvalidID, appErr.Code)
}

// --- Delete ---

func TestDeleteRole_Success(t *testing.T) {
	svc, roleRepo, cacheSvc, txMgr := newRoleTestEnv(t)
	ctx := context.Background()

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	roleRepo.EXPECT().Exists(ctx, 2).Return(true, nil)
	roleRepo.EXPECT().Delete(ctx, 2).Return(nil)

	// rebuildTree
	roleRepo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Role{
		{ID: 1, Name: "admin", ParentID: -1},
	}, nil)
	roleRepo.EXPECT().UpdateBulk(gomock.Any(), gomock.Any()).Return(nil)

	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)

	err := svc.Delete(ctx, 2)
	assert.NoError(t, err)
}

func TestDeleteRole_NotFound(t *testing.T) {
	svc, roleRepo, _, txMgr := newRoleTestEnv(t)
	ctx := context.Background()

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	roleRepo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	require.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
}
