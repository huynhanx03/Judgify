package service

import (
	"context"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/security"
	"github.com/huynhanx03/judgify/pkg/utils"
	"github.com/huynhanx03/judgify/pkg/common/cache"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/identity/constant"
	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	"github.com/huynhanx03/judgify/pkg/oauth"
)

const (
	oauthTokenType          = "oauth-temp"
	resetTokenType          = "reset-password"
	credentialKeyEmail      = "email"
	credentialKeyExternalID = "external_id"
	credentialKeyVerified   = "verified"
)

// oauthTempClaims holds temporary user info from OAuth providers before a local account is created.
type oauthTempClaims struct {
	jwt.RegisteredClaims
	Email      string `json:"email"`
	ExternalID string `json:"external_id"`
	Provider   string `json:"provider"`
	TokenType  string `json:"type"`
}

// resetClaims contains the user identity for the password reset flow.
type resetClaims struct {
	jwt.RegisteredClaims
	UserID    int    `json:"user_id"`
	TokenType string `json:"type"`
}

// getProvider resolves the appropriate OAuth handler based on the provider name.
func (s *authenticationService) getProvider(name string) (oauth.Provider, error) {
	p, ok := s.oauthProviders[name]
	if !ok {
		return nil, apperr.New(response.CodeBadRequest, fmt.Sprintf("unsupported oauth provider: %s", name), nil)
	}
	return p, nil
}

// OAuthCallback handles the redirect from OAuth providers.
func (s *authenticationService) OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error) {
	provider, err := s.getProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	userInfo, err := provider.ExchangeCode(ctx, req.Code)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeBadRequest, "failed to exchange oauth code")
	}

	// Check if this external identity is already linked to an existing user.
	fedIdentity, err := s.fedIdentityRepo.GetByProviderAndExternalID(ctx, req.Provider, userInfo.ExternalID)
	if err == nil && fedIdentity != nil {
		user, err := s.userRepo.Get(ctx, fedIdentity.UserID)
		if err != nil {
			return nil, err
		}

		refreshToken, err := s.generateToken(user, utils.RefreshToken)
		if err != nil {
			return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
		}

		accessToken, err := s.generateToken(user, utils.AccessToken)
		if err != nil {
			return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
		}

		return &dto.OAuthCallbackResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}, nil
	}

	// New user: Issue a short-lived registration ticket.
	email, _ := userInfo.Metadata[credentialKeyEmail].(string)
	tempToken, err := s.generateOAuthTempToken(req.Provider, email, userInfo.ExternalID)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	return &dto.OAuthCallbackResponse{
		RequiresRegistration: true,
		OAuthToken:           tempToken,
	}, nil
}

// OAuthRegister links verified OAuth data with user-chosen credentials.
func (s *authenticationService) OAuthRegister(ctx context.Context, req *dto.OAuthRegisterRequest) (*dto.LoginResponse, error) {
	claims, err := s.parseOAuthTempToken(req.OAuthToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeBadRequest, constant.MsgInvalidGoogleToken)
	}

	if req.Provider != "" && req.Provider != claims.Provider {
		return nil, apperr.New(response.CodeBadRequest, "provider mismatch", nil)
	}

	// Resolve default role for new user
	defaultRole, err := s.resolveRole(ctx, defaultRoleName)
	if err != nil {
		return nil, err
	}

	var user *entity.User

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.registerInternal(ctx, &dto.CreateUserRequest{
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

		oauthCred := &entity.Credential{
			UserID: user.ID,
			Type:   claims.Provider,
			CredentialData: map[string]any{
				credentialKeyEmail:      claims.Email,
				credentialKeyExternalID: claims.ExternalID,
				credentialKeyVerified:   true,
			},
		}
		if err := s.credentialRepo.Create(ctx, oauthCred); err != nil {
			return err
		}

		fedIdentity := &entity.FederatedIdentity{
			UserID:     user.ID,
			Provider:   claims.Provider,
			ExternalID: claims.ExternalID,
		}
		return s.fedIdentityRepo.Create(ctx, fedIdentity)
	})

	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(user, utils.RefreshToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	accessToken, err := s.generateToken(user, utils.AccessToken)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// LinkOAuth allows an authenticated user to connect additional OAuth accounts.
func (s *authenticationService) LinkOAuth(ctx context.Context, userID int, req *dto.OAuthLinkRequest) (*dto.OAuthLinkResponse, error) {
	provider, err := s.getProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	userInfo, err := provider.ExchangeCode(ctx, req.Code)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeBadRequest, "failed to exchange oauth code")
	}

	existing, _ := s.fedIdentityRepo.GetByProviderAndExternalID(ctx, req.Provider, userInfo.ExternalID)
	if existing != nil {
		return nil, apperr.New(response.CodeConflict, constant.MsgGoogleAlreadyUsed, nil)
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		credData := make(map[string]any)
		for k, v := range userInfo.Metadata {
			credData[k] = v
		}

		credData[credentialKeyExternalID] = userInfo.ExternalID
		if _, ok := credData[credentialKeyVerified]; !ok {
			credData[credentialKeyVerified] = true
		}

		oauthCred := &entity.Credential{
			UserID:         userID,
			Type:           req.Provider,
			CredentialData: credData,
		}
		if err := s.credentialRepo.Create(ctx, oauthCred); err != nil {
			return err
		}

		fedIdentity := &entity.FederatedIdentity{
			UserID:     userID,
			Provider:   req.Provider,
			ExternalID: userInfo.ExternalID,
		}
		return s.fedIdentityRepo.Create(ctx, fedIdentity)
	})

	if err != nil {
		return nil, err
	}

	return &dto.OAuthLinkResponse{Success: true}, nil
}

// ForgotPassword initiates recovery by finding a verified email from linked OAuth accounts.
func (s *authenticationService) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
	successResp := &dto.ForgotPasswordResponse{Message: constant.MsgForgotPasswordMsg}

	rateLimitKey := constant.CacheKeyAuthRateLimitForgot + req.Username
	if _, found := cache.GetLocal[string](global.Tinylfu, rateLimitKey); found {
		return nil, apperr.New(response.CodeBadRequest, constant.MsgRateLimitForgot, nil)
	}

	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return successResp, nil
	}

	email := s.findUserEmail(ctx, user.ID)
	if email == "" {
		return successResp, nil
	}

	resetToken, err := s.generateResetToken(user.ID)
	fmt.Println("Reset token:", resetToken)
	if err != nil {
		return successResp, nil
	}

	cache.SetLocalWithTTL(global.Tinylfu, rateLimitKey, "1", 1, constant.ForgotRateLimitTTL)
	// Notification event publication removed
	return successResp, nil
}

// findUserEmail scans linked OAuth credentials for an email address.
func (s *authenticationService) findUserEmail(ctx context.Context, userID int) string {
	for providerName := range s.oauthProviders {
		cred, err := s.credentialRepo.GetByUserID(ctx, userID, providerName)
		if err != nil || cred == nil {
			continue
		}

		email, _ := cred.CredentialData[credentialKeyEmail].(string)
		if email != "" {
			return email
		}
	}
	return ""
}

// ResetPassword validates the reset token and updates the password.
func (s *authenticationService) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	claims, err := s.parseResetToken(req.Token)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeBadRequest, constant.MsgInvalidResetToken)
	}

	blacklistKey := constant.CacheKeyAuthBlacklistJTI + claims.ID
	if _, found := cache.GetLocal[string](global.Tinylfu, blacklistKey); found {
		return nil, apperr.New(response.CodeBadRequest, constant.MsgTokenAlreadyUsed, nil)
	}

	newHash, err := security.HashPassword(req.NewPassword)
	if err != nil {
		return nil, apperr.MapError(err, response.CodeInternalError, apperr.MsgGenFailed)
	}

	err = global.EntClient.DoInTx(ctx, func(ctx context.Context) error {
		cred, err := s.credentialRepo.GetByUserID(ctx, claims.UserID, credentialTypePassword)
		if err != nil {
			return err
		}
		cred.CredentialData[credentialKeyHash] = newHash
		if err := s.credentialRepo.Update(ctx, cred); err != nil {
			return err
		}
		cache.SetLocalWithTTL(global.Tinylfu, blacklistKey, "1", 1, constant.ResetTokenTTL)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &dto.ResetPasswordResponse{Success: true}, nil
}

// generateOAuthTempToken builds a signed JWT to carry OAuth data during registration.
func (s *authenticationService) generateOAuthTempToken(provider, email, externalID string) (string, error) {
	privateKey := global.Config.JWT.PrivateKey
	claims := &oauthTempClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(constant.OAuthTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
		Email:      email,
		ExternalID: externalID,
		Provider:   provider,
		TokenType:  oauthTokenType,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func (s *authenticationService) parseOAuthTempToken(tokenStr string) (*oauthTempClaims, error) {
	publicKey := global.Config.JWT.PublicKey
	claims := &oauthTempClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid oauth temp token: %w", err)
	}
	if claims.TokenType != oauthTokenType {
		return nil, fmt.Errorf("wrong token type: %s", claims.TokenType)
	}

	return claims, nil
}

func (s *authenticationService) generateResetToken(userID int) (string, error) {
	privateKey := global.Config.JWT.PrivateKey
	claims := &resetClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(constant.ResetTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
		UserID:    userID,
		TokenType: resetTokenType,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func (s *authenticationService) parseResetToken(tokenStr string) (*resetClaims, error) {
	publicKey := global.Config.JWT.PublicKey
	return parseResetTokenWithKey(tokenStr, publicKey)
}

func parseResetTokenWithKey(tokenStr string, publicKey *rsa.PublicKey) (*resetClaims, error) {
	claims := &resetClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid reset token: %w", err)
	}
	if claims.TokenType != resetTokenType {
		return nil, fmt.Errorf("wrong token type: %s", claims.TokenType)
	}

	return claims, nil
}
