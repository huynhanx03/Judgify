package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// ElementHandler defines the element HTTP handler interface.
type ElementHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ElementResponse], error)
	FindAll(ctx context.Context, req *dto.FindAllElementsRequest) ([]*dto.ElementResponse, error)
	Get(ctx context.Context, req *dto.GetElementRequest) (*dto.ElementResponse, error)
	Create(ctx context.Context, req *dto.CreateElementRequest) (*dto.ElementResponse, error)
	Update(ctx context.Context, req *dto.UpdateElementRequest) (*dto.ElementResponse, error)
	Delete(ctx context.Context, req *dto.DeleteElementRequest) (*dto.ElementResponse, error)
}

type elementHandler struct {
	handler.BaseHandler
	elementService ports.ElementService
}

func NewElementHandler(svc ports.ElementService) ElementHandler {
	return &elementHandler{elementService: svc}
}

func (h *elementHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ElementResponse], error) {
	return h.elementService.Find(ctx, req)
}

func (h *elementHandler) FindAll(ctx context.Context, _ *dto.FindAllElementsRequest) ([]*dto.ElementResponse, error) {
	return h.elementService.FindAll(ctx)
}

func (h *elementHandler) Get(ctx context.Context, req *dto.GetElementRequest) (*dto.ElementResponse, error) {
	return h.elementService.Get(ctx, req.ID)
}

func (h *elementHandler) Create(ctx context.Context, req *dto.CreateElementRequest) (*dto.ElementResponse, error) {
	return h.elementService.Create(ctx, req)
}

func (h *elementHandler) Update(ctx context.Context, req *dto.UpdateElementRequest) (*dto.ElementResponse, error) {
	return h.elementService.Update(ctx, req.ID, req)
}

func (h *elementHandler) Delete(ctx context.Context, req *dto.DeleteElementRequest) (*dto.ElementResponse, error) {
	return nil, h.elementService.Delete(ctx, req.ID)
}
