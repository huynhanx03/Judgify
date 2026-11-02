package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// UserTraitHandler defines the user trait HTTP handler interface.
type UserTraitHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*userTraitResponse], error)
	Get(ctx context.Context, req *userTraitIDRequest) (*userTraitResponse, error)
}

type userTraitHandler struct {
	handler.BaseHandler
	userTraitService ports.UserTraitService
}

func NewUserTraitHandler(svc ports.UserTraitService) UserTraitHandler {
	return &userTraitHandler{userTraitService: svc}
}

func (h *userTraitHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*userTraitResponse], error) {
	result, err := h.userTraitService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	return toUserTraitPage(result), nil
}

func (h *userTraitHandler) Get(ctx context.Context, req *userTraitIDRequest) (*userTraitResponse, error) {
	id, err := toUserTraitID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.userTraitService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserTraitResponse(result), nil
}
