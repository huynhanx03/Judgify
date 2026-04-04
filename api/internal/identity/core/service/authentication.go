package service

import (
	"context"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/logger"
	"github.com/huynhanx03/judgify/pkg/security"
	"go.uber.org/zap"
	identityUtils "github.com/huynhanx03/judgify/internal/identity/utils"
	"github.com/huynhanx03/judgify/pkg/utils"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	cultivationEntity "github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultivationPorts "github.com/huynhanx03/judgify/internal/cultivation/ports"
	problemPorts "github.com/huynhanx03/judgify/internal/problem/ports"
	"github.com/huynhanx03/judgify/pkg/oauth"
)

const (
	credentialTypePassword = "password"
	defaultRoleName        = "student"
	credentialKeyHash      = "hash"
)

type authenticationService struct {
	userRepo            ports.UserRepository
	credentialRepo      ports.CredentialRepository
	roleRepo            ports.RoleRepository
	permissionRepo      ports.PermissionRepository
	resourceRepo        ports.ResourceRepository
	attrDefRepo         ports.AttributeDefinitionRepository
	attrValueRepo       ports.UserAttributeValueRepository
	fedIdentityRepo     ports.FederatedIdentityRepository
	userTraitRepo       cultivationPorts.UserTraitRepository
	userStatsRepo       cultivationPorts.UserStatsRepository
	userElementExpRepo  cultivationPorts.UserElementExpRepository
	elementRepo         cultivationPorts.ElementRepository
	userDiffStatsRepo   cultivationPorts.UserDifficultyStatsRepository
	difficultyRepo      problemPorts.DifficultyRepository
	oauthProviders      map[string]oauth.Provider
	cache               cache.LocalCache[string, any]
	cacheService        ports.CacheService
}

// NewAuthenticationService creates a new AuthenticationService instance.
func NewAuthenticationService(
	userRepo ports.UserRepository,
	credentialRepo ports.CredentialRepository,
	roleRepo ports.RoleRepository,
	permissionRepo ports.PermissionRepository,
	resourceRepo ports.ResourceRepository,
	attrDefRepo ports.AttributeDefinitionRepository,
	attrValueRepo ports.UserAttributeValueRepository,
	fedIdentityRepo ports.FederatedIdentityRepository,
	userTraitRepo cultivationPorts.UserTraitRepository,
	userStatsRepo cultivationPorts.UserStatsRepository,
	userElementExpRepo cultivationPorts.UserElementExpRepository,
	elementRepo cultivationPorts.ElementRepository,
	userDiffStatsRepo cultivationPorts.UserDifficultyStatsRepository,
	difficultyRepo problemPorts.DifficultyRepository,
	oauthProviders map[string]oauth.Provider,
	cache cache.LocalCache[string, any],
	cacheService ports.CacheService,
) ports.AuthenticationService {
	return &authenticationService{
		userRepo:           userRepo,
		credentialRepo:     credentialRepo,
		roleRepo:           roleRepo,
		permissionRepo:     permissionRepo,
		resourceRepo:       resourceRepo,
		attrDefRepo:        attrDefRepo,
		attrValueRepo:      attrValueRepo,
		fedIdentityRepo:    fedIdentityRepo,
		userTraitRepo:      userTraitRepo,
		userStatsRepo:      userStatsRepo,
		userElementExpRepo: userElementExpRepo,
		elementRepo:        elementRepo,
		userDiffStatsRepo:  userDiffStatsRepo,
		difficultyRepo:     difficultyRepo,
		oauthProviders:     oauthProviders,
		cache:              cache,
		cacheService:       cacheService,
	}
}

// Register creates a new user with default "student" role and optional trait assignments.
func (s *authenticationService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	// Resolve the default role
	defaultRole, err := s.resolveRole(ctx, defaultRoleName)
	if err != nil {
		return nil, err
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		user, err := s.registerInternal(ctx, &dto.CreateUserRequest{
			Username:  req.Username,
			Password:  req.Password,
			RoleID:    defaultRole.ID,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Gender:    req.Gender,
			Birthday:  req.Birthday,
		})
		if err != nil {
			return err
		}

		// Assign traits: 1 root bone + 3 talents
		traitIDs := append([]int{req.RootBoneID}, req.TalentIDs...)
		userTraits := make([]*cultivationEntity.UserTrait, 0, len(traitIDs))
		for _, traitID := range traitIDs {
			userTraits = append(userTraits, &cultivationEntity.UserTrait{
				UserID:  user.ID,
				TraitID: traitID,
			})
		}
		if err := s.userTraitRepo.CreateBulk(ctx, userTraits); err != nil {
			return apperr.MapError(err, response.CodeInternalError, "failed to assign traits")
		}

		// Create user_stats row (level 1, 0 exp)
		if err := s.userStatsRepo.Create(ctx, &cultivationEntity.UserStats{
			UserID:   user.ID,
			TotalExp: 0,
			Rating:   0,
		}); err != nil {
			return apperr.MapError(err, response.CodeInternalError, "failed to init user stats")
		}

		// Create user_element_exp for ALL elements (0 exp each)
		elements, err := s.elementRepo.FindAll(ctx)
		if err != nil {
			return apperr.MapError(err, response.CodeInternalError, "failed to load elements")
		}
		for _, elem := range elements {
			if err := s.userElementExpRepo.Create(ctx, &cultivationEntity.UserElementExp{
				UserID:    user.ID,
				ElementID: elem.ID,
				Exp:       0,
			}); err != nil {
				return apperr.MapError(err, response.CodeInternalError, "failed to init element exp")
			}
		}

		// Create user_difficulty_stats for ALL difficulties (0 solved each)
		difficulties, err := s.difficultyRepo.FindAll(ctx)
		if err != nil {
			return apperr.MapError(err, response.CodeInternalError, "failed to load difficulties")
		}
		diffIDs := make([]int, len(difficulties))
		for i, d := range difficulties {
			diffIDs[i] = d.ID
		}
		if err := s.userDiffStatsRepo.CreateBulk(ctx, user.ID, diffIDs); err != nil {
			return apperr.MapError(err, response.CodeInternalError, "failed to init difficulty stats")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("new user registered", zap.String("username", req.Username))

	return &dto.RegisterResponse{Success: true}, nil
}

// registerInternal handles the core logic of creating a user.
// Used by both standard Register and OAuthRegister flows.
func (s *authenticationService) registerInternal(ctx context.Context, req *dto.CreateUserRequest) (*entity.User, error) {
	user, err := s.CreateUser(ctx, req)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Login processes user login and returns JWT tokens.
func (s *authenticationService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	cred, err := s.credentialRepo.GetByUserID(ctx, user.ID, credentialTypePassword)
	if err != nil {
		return nil, err
	}

	hash, ok := cred.CredentialData[credentialKeyHash].(string)
	if !ok {
		return nil, apperr.New(response.CodeInternalError, constant.MsgInvalidCredData, nil)
	}

	if err := security.ComparePassword(hash, req.Password); err != nil {
		logger.FromContext(ctx).Warn("invalid login attempt: password mismatch", zap.String("username", req.Username))
		return nil, apperr.MapError(err, response.CodeUnauthorized, constant.MsgInvalidAuth)
	}

	refreshToken, err := s.generateToken(user, utils.RefreshToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	accessToken, err := s.generateToken(user, utils.AccessToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	logger.FromContext(ctx).Info("user logged in", zap.Int("user_id", user.ID), zap.String("username", user.Username))

	return &dto.LoginResponse{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
	}, nil
}

// ChangePassword processes password change request.
func (s *authenticationService) ChangePassword(ctx context.Context, userID int, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	cred, err := s.credentialRepo.GetByUserID(ctx, userID, credentialTypePassword)
	if err != nil {
		return nil, err
	}

	hash, ok := cred.CredentialData[credentialKeyHash].(string)
	if !ok {
		return nil, apperr.New(response.CodeInternalError, constant.MsgInvalidCredData, nil)
	}

	if err := security.ComparePassword(hash, req.CurrentPassword); err != nil {
		logger.FromContext(ctx).Warn("invalid current password provided for change password", zap.Int("user_id", userID))
		return nil, apperr.MapError(err, response.CodeUnauthorized, constant.MsgPassIncorrect)
	}

	newHash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		cred.CredentialData[credentialKeyHash] = newHash
		return s.credentialRepo.Update(ctx, cred)
	})

	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info("user changed password successfully", zap.Int("user_id", userID))

	return &dto.ChangePasswordResponse{Success: true}, nil
}

// CreateUser creates a new user with the specified role.
func (s *authenticationService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*entity.User, error) {
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.New(response.CodeConflict, constant.MsgUsernameExists, nil)
	}

	var user *entity.User
	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		user = &entity.User{
			Username: req.Username,
			RoleID:   req.RoleID,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return err
		}

		hashedPassword, err := security.HashPassword(req.Password)
		if err != nil {
			return apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
		}

		credential := &entity.Credential{
			UserID: user.ID,
			Type:   credentialTypePassword,
			CredentialData: map[string]any{
				credentialKeyHash: hashedPassword,
			},
		}

		if err := s.credentialRepo.Create(ctx, credential); err != nil {
			return err
		}

		attrKeys := map[string]string{
			constant.AttributeKeyFirstName: req.FirstName,
			constant.AttributeKeyLastName:  req.LastName,
			constant.AttributeKeyGender:    strconv.Itoa(req.Gender),
			constant.AttributeKeyBirthday:  req.Birthday,
		}

		var attrValues []*entity.UserAttributeValue
		for key, value := range attrKeys {
			if value == "" {
				continue
			}

			def, err := identityUtils.GetAttributeDefinition(ctx, key, s.attrDefRepo, s.cache)
			if err != nil {
				return err
			}

			attrValues = append(attrValues, &entity.UserAttributeValue{
				UserID:      user.ID,
				AttributeID: def.ID,
				Value:       value,
			})
		}

		if len(attrValues) > 0 {
			if err := s.attrValueRepo.CreateBulk(ctx, attrValues); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}

// RefreshToken refreshes the access token.
func (s *authenticationService) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest, userID int) (*dto.RefreshTokenResponse, error) {
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	token, err := s.generateToken(user, utils.AccessToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	return &dto.RefreshTokenResponse{
		AccessToken: token,
	}, nil
}

// generateToken generates a JWT token (access or refresh) with only UserID + Username.
func (s *authenticationService) generateToken(user *entity.User, tokenType utils.TokenType) (string, error) {
	privateKey := global.Config.JWT.PrivateKey
	return utils.GenerateToken(privateKey, user.ID, user.Username, tokenType)
}

// resolveRole fetches a role by name with local caching.
func (s *authenticationService) resolveRole(ctx context.Context, roleName string) (*entity.Role, error) {
	cacheKey := constant.CacheKeyPrefixRoleName + roleName

	if role, found := cache.LocalGet[*entity.Role](s.cache, cacheKey); found {
		return role, nil
	}

	role, err := s.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return nil, err
	}

	cache.LocalSet(s.cache, cacheKey, role)
	return role, nil
}
