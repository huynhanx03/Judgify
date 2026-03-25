package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// UserTraitHandler defines the user trait HTTP handler interface.
type UserTraitHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserTraitResponse], error)
	Get(ctx context.Context, req *dto.GetUserTraitRequest) (*dto.UserTraitResponse, error)
	Create(ctx context.Context, req *dto.CreateUserTraitRequest) (*dto.UserTraitResponse, error)
	Delete(ctx context.Context, req *dto.DeleteUserTraitRequest) (*dto.UserTraitResponse, error)
}

type userTraitHandler struct {
	handler.BaseHandler
	userTraitService ports.UserTraitService
}

func NewUserTraitHandler(svc ports.UserTraitService) UserTraitHandler {
	return &userTraitHandler{userTraitService: svc}
}

func (h *userTraitHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserTraitResponse], error) {
	return h.userTraitService.Find(ctx, req)
}

func (h *userTraitHandler) Get(ctx context.Context, req *dto.GetUserTraitRequest) (*dto.UserTraitResponse, error) {
	return h.userTraitService.Get(ctx, req.ID)
}

func (h *userTraitHandler) Create(ctx context.Context, req *dto.CreateUserTraitRequest) (*dto.UserTraitResponse, error) {
	return h.userTraitService.Create(ctx, req)
}

func (h *userTraitHandler) Delete(ctx context.Context, req *dto.DeleteUserTraitRequest) (*dto.UserTraitResponse, error) {
	return nil, h.userTraitService.Delete(ctx, req.ID)
}
