package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

type UserHandler interface {
	Create(ctx context.Context, req *dto.CreateUserRequest) (*dto.SuccessResponse, error)
	Delete(ctx context.Context, req *dto.DeleteUserRequest) (*dto.SuccessResponse, error)
	UpdateProfile(ctx context.Context, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	GetProfile(ctx context.Context, req *dto.GetProfileRequest) (*dto.ProfileResponse, error)
}

type userHandler struct {
	userService ports.UserService
	authService ports.AuthenticationService
}

func NewUserHandler(userService ports.UserService, authService ports.AuthenticationService) UserHandler {
	return &userHandler{
		userService: userService,
		authService: authService,
	}
}

func (h *userHandler) Create(ctx context.Context, req *dto.CreateUserRequest) (*dto.SuccessResponse, error) {
	_, err := h.authService.CreateUser(ctx, req)
	if err != nil {
		return nil, err
	}
	return &dto.SuccessResponse{Success: true}, nil
}

func (h *userHandler) Delete(ctx context.Context, req *dto.DeleteUserRequest) (*dto.SuccessResponse, error) {
	err := h.userService.Delete(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return &dto.SuccessResponse{Success: true}, nil
}

func (h *userHandler) UpdateProfile(ctx context.Context, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	if userIDVal == nil {
		return nil, apperr.New(response.CodeUnauthorized, "user id not found in context", nil)
	}
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeInternalError, "invalid user id type", nil)
	}

	return h.userService.UpdateProfile(ctx, userID, req)
}

func (h *userHandler) GetProfile(ctx context.Context, req *dto.GetProfileRequest) (*dto.ProfileResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	if userIDVal == nil {
		return nil, apperr.New(response.CodeUnauthorized, "user id not found in context", nil)
	}
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeInternalError, "invalid user id type", nil)
	}

	return h.userService.GetProfile(ctx, userID)
}
