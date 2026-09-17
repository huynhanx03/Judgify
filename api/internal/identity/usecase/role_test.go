package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	idMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newRoleTestEnv(t *testing.T) (
	*roleService,
	*idMocks.MockRoleRepository,
	*idMocks.MockAuthorizationService,
	*idMocks.MockCacheService,
	testutil.TxManager,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	authzSvc := idMocks.NewMockAuthorizationService(ctrl)
	cacheSvc := idMocks.NewMockCacheService(ctrl)
	txMgr := testutil.TxManager{}

	caches := appcache.New(testutil.NewCache())
	svc := NewRoleService(roleRepo, authzSvc, cacheSvc, txMgr, caches.IdentityRole).(*roleService)
	return svc, roleRepo, authzSvc, cacheSvc, txMgr
}

func TestFindAllRoles_Success(t *testing.T) {
	svc, roleRepo, _, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().FindAll(ctx).Return([]*entity.Role{
		{ID: testutil.ID(1), Key: "admin", Name: "Admin", IsSystem: true},
		{ID: testutil.ID(2), Key: "student", Name: "Student", IsSystem: true},
	}, nil)

	roles, err := svc.FindAll(ctx)

	assert.NoError(t, err)
	assert.Len(t, roles, 2)
	assert.Equal(t, "admin", roles[0].Key)
	assert.Equal(t, "Student", roles[1].Name)
}

func TestGetRole_NotFound(t *testing.T) {
	svc, roleRepo, _, _, _ := newRoleTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().Get(ctx, testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "role not found", nil))

	role, err := svc.Get(ctx, testutil.ID(999))

	assert.Nil(t, role)
	assert.Error(t, err)
}

func TestSetRolePolicies_ReplacesCasbinPolicies(t *testing.T) {
	svc, roleRepo, authzSvc, cacheSvc, _ := newRoleTestEnv(t)
	ctx := context.Background()
	rules := []dto.PolicyRule{{Resource: "problem", Action: "create"}}
	policySnapshot := &dto.RolePolicySnapshot{Revision: 2, Rules: rules}

	roleRepo.EXPECT().Get(ctx, testutil.ID(2)).Return(&entity.Role{ID: testutil.ID(2), Key: "teacher"}, nil)
	authzSvc.EXPECT().SetRolePolicies(gomock.Any(), "teacher", int64(1), "Grant problem creation", rules).Return(policySnapshot, nil)
	cacheSvc.EXPECT().InvalidatePermissionConfig(gomock.Any()).Return(nil)

	resp, err := svc.SetPolicies(ctx, testutil.ID(2), &dto.RolePoliciesUpdateRequest{
		ExpectedRevision: 1,
		Reason:           "Grant problem creation",
		Rules:            rules,
	})

	assert.NoError(t, err)
	assert.Equal(t, policySnapshot, resp)
}
