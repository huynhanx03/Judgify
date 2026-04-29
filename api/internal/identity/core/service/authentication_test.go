package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	idMocks "github.com/huynhanx03/judgify/internal/identity/mocks"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	problemMocks "github.com/huynhanx03/judgify/internal/problem/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
	txMocks "github.com/huynhanx03/judgify/pkg/common/tx/mocks"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/security"
)

// newAuthTestEnv sets up mocks and returns a ready-to-use authenticationService.
func newAuthTestEnv(t *testing.T) (
	*authenticationService,
	*idMocks.MockUserRepository,
	*idMocks.MockCredentialRepository,
	*idMocks.MockRoleRepository,
	*idMocks.MockPermissionRepository,
	*idMocks.MockResourceRepository,
	*idMocks.MockAttributeDefinitionRepository,
	*idMocks.MockUserAttributeValueRepository,
	*idMocks.MockFederatedIdentityRepository,
	*cultMocks.MockUserTraitRepository,
	*cultMocks.MockUserStatsRepository,
	*cultMocks.MockUserElementExpRepository,
	*cultMocks.MockElementRepository,
	*cultMocks.MockUserDifficultyStatsRepository,
	*problemMocks.MockDifficultyRepository,
	*idMocks.MockCacheService,
	*txMocks.MockManager,
	cache.LocalCache[string, any],
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	setupGlobalConfig(t)

	userRepo := idMocks.NewMockUserRepository(ctrl)
	credRepo := idMocks.NewMockCredentialRepository(ctrl)
	roleRepo := idMocks.NewMockRoleRepository(ctrl)
	permRepo := idMocks.NewMockPermissionRepository(ctrl)
	resRepo := idMocks.NewMockResourceRepository(ctrl)
	attrDefRepo := idMocks.NewMockAttributeDefinitionRepository(ctrl)
	attrValRepo := idMocks.NewMockUserAttributeValueRepository(ctrl)
	fedRepo := idMocks.NewMockFederatedIdentityRepository(ctrl)
	userTraitRepo := cultMocks.NewMockUserTraitRepository(ctrl)
	userStatsRepo := cultMocks.NewMockUserStatsRepository(ctrl)
	userElExpRepo := cultMocks.NewMockUserElementExpRepository(ctrl)
	elementRepo := cultMocks.NewMockElementRepository(ctrl)
	userDiffStatsRepo := cultMocks.NewMockUserDifficultyStatsRepository(ctrl)
	diffRepo := problemMocks.NewMockDifficultyRepository(ctrl)
	cacheSvc := idMocks.NewMockCacheService(ctrl)
	txMgr := txMocks.NewMockManager(ctrl)

	// Use a real cache for tests
	localCache := ember.New[string, any]()

	svc := NewAuthenticationService(
		userRepo, credRepo, roleRepo, permRepo, resRepo,
		attrDefRepo, attrValRepo, fedRepo,
		userTraitRepo, userStatsRepo, userElExpRepo, elementRepo, userDiffStatsRepo,
		diffRepo,
		nil, // oauthProviders — not needed for basic tests
		localCache,
		cacheSvc,
		txMgr,
	).(*authenticationService)

	return svc, userRepo, credRepo, roleRepo, permRepo, resRepo,
		attrDefRepo, attrValRepo, fedRepo,
		userTraitRepo, userStatsRepo, userElExpRepo, elementRepo, userDiffStatsRepo,
		diffRepo, cacheSvc, txMgr, localCache
}

// --- Login Tests ---

func TestLogin_Success(t *testing.T) {
	svc, userRepo, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	hashedPassword, err := security.HashPassword("password123")
	require.NoError(t, err)

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   1,
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(&entity.Credential{
		ID:     1,
		UserID: 1,
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username: "testuser",
		Password: "password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, userRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "nonexistent").Return(nil, apperr.New(response.CodeNotFound, "user not found", nil))

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_InvalidPassword(t *testing.T) {
	svc, userRepo, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	hashedPassword, err := security.HashPassword("correct_password")
	require.NoError(t, err)

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   1,
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(&entity.Credential{
		ID:     1,
		UserID: 1,
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username: "testuser",
		Password: "wrong_password",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_CredentialNotFound(t *testing.T) {
	svc, userRepo, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   1,
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(nil, apperr.New(response.CodeNotFound, "credential not found", nil))

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username: "testuser",
		Password: "password123",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestLogin_InvalidCredentialData(t *testing.T) {
	svc, userRepo, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	userRepo.EXPECT().GetByUsername(ctx, "testuser").Return(&entity.User{
		ID:       1,
		Username: "testuser",
		RoleID:   1,
	}, nil)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(&entity.Credential{
		ID:     1,
		UserID: 1,
		Type:   "password",
		CredentialData: map[string]any{
			"hash": 12345, // not a string
		},
	}, nil)

	resp, err := svc.Login(ctx, &dto.LoginRequest{
		Username: "testuser",
		Password: "password123",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- ChangePassword Tests ---

func TestChangePassword_Success(t *testing.T) {
	svc, _, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, txMgr, _ := newAuthTestEnv(t)
	ctx := context.Background()

	hashedPassword, err := security.HashPassword("old_password")
	require.NoError(t, err)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(&entity.Credential{
		ID:     1,
		UserID: 1,
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)

	credRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.ChangePassword(ctx, 1, &dto.ChangePasswordRequest{
		CurrentPassword: "old_password",
		NewPassword:     "new_password123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	svc, _, credRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	hashedPassword, err := security.HashPassword("correct_password")
	require.NoError(t, err)

	credRepo.EXPECT().GetByUserID(ctx, 1, "password").Return(&entity.Credential{
		ID:     1,
		UserID: 1,
		Type:   "password",
		CredentialData: map[string]any{
			"hash": hashedPassword,
		},
	}, nil)

	resp, err := svc.ChangePassword(ctx, 1, &dto.ChangePasswordRequest{
		CurrentPassword: "wrong_password",
		NewPassword:     "new_password123",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Register Tests ---

func TestRegister_UsernameExists(t *testing.T) {
	svc, userRepo, _, roleRepo, _, _, _, _, _, _, _, _, _, _, _, _, txMgr, _ := newAuthTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().GetByName(gomock.Any(), "student").Return(&entity.Role{
		ID:   1,
		Name: "student",
	}, nil)

	// Register wraps everything in DoInTx, and inside tx, CreateUser checks ExistsByUsername
	txMgr.EXPECT().DoInTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)
	userRepo.EXPECT().ExistsByUsername(gomock.Any(), "taken_user").Return(true, nil)

	resp, err := svc.Register(ctx, &dto.RegisterRequest{
		Username:   "taken_user",
		Password:   "password123",
		FirstName:  "Test",
		LastName:   "User",
		Gender:     0,
		Birthday:   "2000-01-01",
		RootBoneID: 1,
		TalentIDs:  []int{2, 3, 4},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	require.True(t, ok)
	assert.Equal(t, response.CodeConflict, appErr.Code)
	assert.Equal(t, constant.MsgUsernameExists, appErr.Message)
}

func TestRegister_RoleNotFound(t *testing.T) {
	svc, _, _, roleRepo, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := newAuthTestEnv(t)
	ctx := context.Background()

	roleRepo.EXPECT().GetByName(gomock.Any(), "student").Return(nil, apperr.New(response.CodeNotFound, "role not found", nil))

	resp, err := svc.Register(ctx, &dto.RegisterRequest{
		Username:   "newuser",
		Password:   "password123",
		FirstName:  "Test",
		LastName:   "User",
		Gender:     0,
		Birthday:   "2000-01-01",
		RootBoneID: 1,
		TalentIDs:  []int{2, 3, 4},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// Ensure cultivationPorts import is used (needed by constructor signature)
var _ cultivationPorts.UserTraitRepository = (*cultMocks.MockUserTraitRepository)(nil)
