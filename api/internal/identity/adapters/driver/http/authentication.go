package http

import (
	"context"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/authctx"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AuthenticationHandler defines the authentication HTTP handler interface.
type AuthenticationHandler interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	ChangePassword(ctx context.Context, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error)
	RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error)
	Logout(ctx context.Context) (*dto.LogoutResponse, error)
	OAuthStart(ctx context.Context, req *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error)
	OAuthLinkStart(ctx context.Context, req *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error)
	OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error)
	OAuthInspectLogin(ctx context.Context, req *dto.OAuthLoginInspectionRequest) (*dto.OAuthLoginInspectionResponse, error)
	OAuthFinalizeLogin(ctx context.Context, req *dto.OAuthFinalizeLoginRequest) (*dto.OAuthFinalizeLoginResponse, error)
	OAuthFinalizeLink(ctx context.Context, req *dto.OAuthFinalizeLinkRequest) (*dto.OAuthFinalizeLinkResponse, error)
	OAuthIdentities(ctx context.Context) (*oauthIdentityListResponse, error)
	OAuthUnlink(ctx context.Context, req *oauthProviderRequest) (*oauthUnlinkResponse, error)
	ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error)
	ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
}

type authenticationHandler struct {
	handler.BaseHandler
	authService ports.AuthenticationService
}

// NewAuthenticationHandler creates a new AuthenticationHandler instance.
func NewAuthenticationHandler(authService ports.AuthenticationService) AuthenticationHandler {
	return &authenticationHandler{
		authService: authService,
	}
}

func (h *authenticationHandler) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	return h.authService.Register(ctx, req)
}

func (h *authenticationHandler) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	return h.authService.Login(ctx, req)
}

func (h *authenticationHandler) ChangePassword(ctx context.Context, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	userID, err := h.getUserID(ctx)
	if err != nil {
		return nil, err
	}
	sessionID, ok := authctx.SessionID(ctx)
	if !ok || req == nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "session id not found in context", nil)
	}
	req.SessionID = sessionID
	return h.authService.ChangePassword(ctx, userID, req)
}

func (h *authenticationHandler) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	return h.authService.RefreshToken(ctx, req)
}

func (h *authenticationHandler) Logout(ctx context.Context) (*dto.LogoutResponse, error) {
	sessionID, ok := authctx.SessionID(ctx)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "session id not found in context", nil)
	}
	return h.authService.Logout(ctx, sessionID)
}

func (h *authenticationHandler) OAuthStart(ctx context.Context, req *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error) {
	return h.authService.OAuthStart(ctx, req)
}

func (h *authenticationHandler) OAuthLinkStart(ctx context.Context, req *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error) {
	sessionID, ok := authctx.SessionID(ctx)
	if !ok || req == nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "session id not found in context", nil)
	}
	req.SessionID = sessionID
	return h.authService.OAuthStart(ctx, req)
}

func (h *authenticationHandler) OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error) {
	return h.authService.OAuthCallback(ctx, req)
}

func (h *authenticationHandler) OAuthInspectLogin(ctx context.Context, req *dto.OAuthLoginInspectionRequest) (*dto.OAuthLoginInspectionResponse, error) {
	return h.authService.OAuthInspectLogin(ctx, req)
}

func (h *authenticationHandler) OAuthFinalizeLogin(ctx context.Context, req *dto.OAuthFinalizeLoginRequest) (*dto.OAuthFinalizeLoginResponse, error) {
	return h.authService.OAuthFinalizeLogin(ctx, req)
}

func (h *authenticationHandler) OAuthFinalizeLink(ctx context.Context, req *dto.OAuthFinalizeLinkRequest) (*dto.OAuthFinalizeLinkResponse, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK || req == nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "active session not found in context", nil)
	}
	req.UserID = userID
	req.SessionID = sessionID
	return h.authService.OAuthFinalizeLink(ctx, req)
}

func (h *authenticationHandler) OAuthIdentities(ctx context.Context) (*oauthIdentityListResponse, error) {
	userID, err := h.getUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := h.authService.OAuthIdentities(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toOAuthIdentityListResponse(result), nil
}

func (h *authenticationHandler) OAuthUnlink(ctx context.Context, req *oauthProviderRequest) (*oauthUnlinkResponse, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK || req == nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "active session not found in context", nil)
	}
	result, err := h.authService.OAuthUnlink(ctx, userID, sessionID, toOAuthProviderRequest(req).Provider)
	if err != nil {
		return nil, err
	}
	return toOAuthUnlinkResponse(result), nil
}

func (h *authenticationHandler) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
	return h.authService.ForgotPassword(ctx, req)
}

func (h *authenticationHandler) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	return h.authService.ResetPassword(ctx, req)
}

func (h *authenticationHandler) getUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return uuid.Nil, apperr.New(apperr.CodeUnauthorized, "user id not found in context", nil)
	}
	return userID, nil
}
