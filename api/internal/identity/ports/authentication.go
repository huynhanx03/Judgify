package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// AuthenticationService defines the authentication business logic interface.
type AuthenticationService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	ChangePassword(ctx context.Context, userID int, req *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error)
	CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*entity.User, error)
	RefreshToken(ctx context.Context, req *dto.RefreshTokenRequest, userID int) (*dto.RefreshTokenResponse, error)

	// OAuth
	OAuthCallback(ctx context.Context, req *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error)
	OAuthRegister(ctx context.Context, req *dto.OAuthRegisterRequest) (*dto.LoginResponse, error)
	LinkOAuth(ctx context.Context, userID int, req *dto.OAuthLinkRequest) (*dto.OAuthLinkResponse, error)

	// Password Recovery
	ForgotPassword(ctx context.Context, req *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error)
	ResetPassword(ctx context.Context, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
}
