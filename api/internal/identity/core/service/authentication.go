package service

import (
	"context"
	"net/http"
	"strconv"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/security"
	identityUtils "github.com/huynhanx03/judgify/internal/identity/utils"
	"github.com/huynhanx03/judgify/pkg/utils"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/internal/identity/ports"
	"github.com/huynhanx03/judgify/pkg/oauth"
)

const (
	authServiceName = "AuthenticationService"

	credentialTypePassword = "password"
	defaultRoleName        = "student"
	credentialKeyHash      = "hash"
)

type authenticationService struct {
	userRepo        ports.UserRepository
	credentialRepo  ports.CredentialRepository
	roleRepo        ports.RoleRepository
	permissionRepo  ports.PermissionRepository
	resourceRepo    ports.ResourceRepository
	attrDefRepo     ports.AttributeDefinitionRepository
	attrValueRepo   ports.UserAttributeValueRepository
	fedIdentityRepo ports.FederatedIdentityRepository
	oauthProviders  map[string]oauth.Provider
	cache           cache.LocalCache[string, any]
	cacheService    ports.CacheService
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
	oauthProviders map[string]oauth.Provider,
	cache cache.LocalCache[string, any],
	cacheService ports.CacheService,
) ports.AuthenticationService {
	return &authenticationService{
		userRepo:        userRepo,
		credentialRepo:  credentialRepo,
		roleRepo:        roleRepo,
		permissionRepo:  permissionRepo,
		resourceRepo:    resourceRepo,
		attrDefRepo:     attrDefRepo,
		attrValueRepo:   attrValueRepo,
		fedIdentityRepo: fedIdentityRepo,
		oauthProviders:  oauthProviders,
		cache:           cache,
		cacheService:    cacheService,
	}
}

// Register creates a new user with default "student" role.
func (s *authenticationService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	// Resolve the default role
	defaultRole, err := s.resolveRole(ctx, defaultRoleName)
	if err != nil {
		return nil, err
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		_, err := s.registerInternal(ctx, &dto.CreateUserRequest{
			Username:  req.Username,
			Password:  req.Password,
			RoleID:    defaultRole.ID,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Gender:    req.Gender,
			Birthday:  req.Birthday,
		})
		return err
	})

	if err != nil {
		return nil, err
	}

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
		return nil, apperr.NewError(authServiceName, response.CodeInternalError, constant.MsgInvalidCredData, http.StatusInternalServerError, nil)
	}

	if err := security.ComparePassword(hash, req.Password); err != nil {
		return nil, apperr.MapError(authServiceName, err, response.CodeUnauthorized, constant.MsgInvalidAuth, http.StatusUnauthorized)
	}

	refreshToken, err := s.generateToken(user, utils.RefreshToken)
	if err != nil {
		return nil, apperr.MapError(authServiceName, err, response.CodeInternalError, apperr.MsgGenFailed, http.StatusInternalServerError)
	}

	accessToken, err := s.generateToken(user, utils.AccessToken)
	if err != nil {
		return nil, apperr.MapError(authServiceName, err, response.CodeInternalError, apperr.MsgGenFailed, http.StatusInternalServerError)
	}

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
		return nil, apperr.NewError(authServiceName, response.CodeInternalError, constant.MsgInvalidCredData, http.StatusInternalServerError, nil)
	}

	if err := security.ComparePassword(hash, req.CurrentPassword); err != nil {
		return nil, apperr.MapError(authServiceName, err, response.CodeUnauthorized, constant.MsgPassIncorrect, http.StatusUnauthorized)
	}

	newHash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		return nil, apperr.MapError(authServiceName, err, response.CodeInternalError, apperr.MsgGenFailed, http.StatusInternalServerError)
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		cred.CredentialData[credentialKeyHash] = newHash
		return s.credentialRepo.Update(ctx, cred)
	})

	if err != nil {
		return nil, err
	}

	return &dto.ChangePasswordResponse{Success: true}, nil
}

// CreateUser creates a new user with the specified role.
func (s *authenticationService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*entity.User, error) {
	exists, err := s.userRepo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.NewError(authServiceName, response.CodeConflict, constant.MsgUsernameExists, http.StatusConflict, nil)
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
			return apperr.MapError(authServiceName, err, response.CodeInternalError, apperr.MsgGenFailed, http.StatusInternalServerError)
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
		return nil, apperr.MapError(authServiceName, err, response.CodeInternalError, apperr.MsgGenFailed, http.StatusInternalServerError)
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

	if role, found := cache.GetLocal[*entity.Role](s.cache, cacheKey); found {
		return role, nil
	}

	role, err := s.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return nil, err
	}

	cache.SetLocal(s.cache, cacheKey, role, constant.CacheCostRoleName)
	return role, nil
}
