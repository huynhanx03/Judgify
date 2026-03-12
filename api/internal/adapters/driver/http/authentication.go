package http

import (
	"context"
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"

	"github.com/huynhanx03/judgify/internal/core/dto"
	"github.com/huynhanx03/judgify/internal/ports"
)

// AuthenticationHandler defines the authentication HTTP handler interface.
type AuthenticationHandler interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	ChangePassword(ctx context.Context, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error)
	RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error)
	OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error)
	OAuthRegister(ctx context.Context, req *dto.OAuthRegisterRequest) (*dto.LoginResponse, error)
	LinkOAuth(ctx context.Context, req *dto.OAuthLinkRequest) (*dto.OAuthLinkResponse, error)
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
	return h.authService.ChangePassword(ctx, userID, req)
}

func (h *authenticationHandler) RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	userID, err := h.getUserID(ctx)
	if err != nil {
		return nil, err
	}
	return h.authService.RefreshToken(ctx, req, userID)
}

func (h *authenticationHandler) OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error) {
	return h.authService.OAuthCallback(ctx, req)
}

func (h *authenticationHandler) OAuthRegister(ctx context.Context, req *dto.OAuthRegisterRequest) (*dto.LoginResponse, error) {
	return h.authService.OAuthRegister(ctx, req)
}

func (h *authenticationHandler) LinkOAuth(ctx context.Context, req *dto.OAuthLinkRequest) (*dto.OAuthLinkResponse, error) {
	userID, err := h.getUserID(ctx)
	if err != nil {
		return nil, err
	}
	return h.authService.LinkOAuth(ctx, userID, req)
}

func (h *authenticationHandler) ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
	return h.authService.ForgotPassword(ctx, req)
}

func (h *authenticationHandler) ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	return h.authService.ResetPassword(ctx, req)
}

func (h *authenticationHandler) getUserID(ctx context.Context) (int, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	if userIDVal == nil {
		return 0, apperr.New(response.CodeUnauthorized, "user id not found in context", http.StatusUnauthorized, nil)
	}

	userID, ok := userIDVal.(int)
	if !ok {
		return 0, apperr.New(response.CodeInternalError, "invalid user id type", http.StatusInternalServerError, nil)
	}
	return userID, nil
}
