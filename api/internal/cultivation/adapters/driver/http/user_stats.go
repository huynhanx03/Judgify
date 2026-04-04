package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// UserStatsHandler defines the user stats HTTP handler interface.
type UserStatsHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserStatsResponse], error)
	Get(ctx context.Context, req *dto.GetUserStatsRequest) (*dto.UserStatsResponse, error)
	Create(ctx context.Context, req *dto.CreateUserStatsRequest) (*dto.UserStatsResponse, error)
	Update(ctx context.Context, req *dto.UpdateUserStatsRequest) (*dto.UserStatsResponse, error)
	Delete(ctx context.Context, req *dto.DeleteUserStatsRequest) (*dto.UserStatsResponse, error)
}

type userStatsHandler struct {
	handler.BaseHandler
	userStatsService ports.UserStatsService
}

func NewUserStatsHandler(svc ports.UserStatsService) UserStatsHandler {
	return &userStatsHandler{userStatsService: svc}
}

func (h *userStatsHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserStatsResponse], error) {
	return h.userStatsService.Find(ctx, req)
}

func (h *userStatsHandler) Get(ctx context.Context, req *dto.GetUserStatsRequest) (*dto.UserStatsResponse, error) {
	return h.userStatsService.Get(ctx, req.ID)
}

func (h *userStatsHandler) Create(ctx context.Context, req *dto.CreateUserStatsRequest) (*dto.UserStatsResponse, error) {
	return h.userStatsService.Create(ctx, req)
}

func (h *userStatsHandler) Update(ctx context.Context, req *dto.UpdateUserStatsRequest) (*dto.UserStatsResponse, error) {
	return h.userStatsService.Update(ctx, req.ID, req)
}

func (h *userStatsHandler) Delete(ctx context.Context, req *dto.DeleteUserStatsRequest) (*dto.UserStatsResponse, error) {
	return nil, h.userStatsService.Delete(ctx, req.ID)
}
