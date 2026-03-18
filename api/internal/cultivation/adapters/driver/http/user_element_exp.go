package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// UserElementExpHandler defines the user element EXP HTTP handler interface.
type UserElementExpHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserElementExpResponse], error)
	Get(ctx context.Context, req *dto.GetUserElementExpRequest) (*dto.UserElementExpResponse, error)
	Create(ctx context.Context, req *dto.CreateUserElementExpRequest) (*dto.UserElementExpResponse, error)
	Update(ctx context.Context, req *dto.UpdateUserElementExpRequest) (*dto.UserElementExpResponse, error)
	Delete(ctx context.Context, req *dto.DeleteUserElementExpRequest) (*dto.UserElementExpResponse, error)
}

type userElementExpHandler struct {
	handler.BaseHandler
	userElementExpService ports.UserElementExpService
}

func NewUserElementExpHandler(svc ports.UserElementExpService) UserElementExpHandler {
	return &userElementExpHandler{userElementExpService: svc}
}

func (h *userElementExpHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.UserElementExpResponse], error) {
	return h.userElementExpService.Find(ctx, req)
}

func (h *userElementExpHandler) Get(ctx context.Context, req *dto.GetUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	return h.userElementExpService.Get(ctx, req.ID)
}

func (h *userElementExpHandler) Create(ctx context.Context, req *dto.CreateUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	return h.userElementExpService.Create(ctx, req)
}

func (h *userElementExpHandler) Update(ctx context.Context, req *dto.UpdateUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	return h.userElementExpService.Update(ctx, req.ID, req)
}

func (h *userElementExpHandler) Delete(ctx context.Context, req *dto.DeleteUserElementExpRequest) (*dto.UserElementExpResponse, error) {
	return nil, h.userElementExpService.Delete(ctx, req.ID)
}
