package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// ElementHandler defines the element HTTP handler interface.
type ElementHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*elementResponse], error)
	FindAll(ctx context.Context) ([]*elementResponse, error)
	Get(ctx context.Context, req *elementIDRequest) (*elementResponse, error)
	Create(ctx context.Context, req *createElementRequest) (*elementResponse, error)
	Update(ctx context.Context, req *updateElementRequest) (*elementResponse, error)
	Delete(ctx context.Context, req *elementIDRequest) (*elementResponse, error)
}

type elementHandler struct {
	handler.BaseHandler
	elementService ports.ElementService
}

func NewElementHandler(svc ports.ElementService) ElementHandler {
	return &elementHandler{elementService: svc}
}

func (h *elementHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*elementResponse], error) {
	result, err := h.elementService.Find(ctx, req)
	return toElementPage(result), err
}

func (h *elementHandler) FindAll(ctx context.Context) ([]*elementResponse, error) {
	result, err := h.elementService.FindAll(ctx)
	return toElementResponses(result), err
}

func (h *elementHandler) Get(ctx context.Context, req *elementIDRequest) (*elementResponse, error) {
	id, err := toElementID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.elementService.Get(ctx, id)
	return toElementResponse(result), err
}

func (h *elementHandler) Create(ctx context.Context, req *createElementRequest) (*elementResponse, error) {
	result, err := h.elementService.Create(ctx, toCreateElementCommand(req))
	return toElementResponse(result), err
}

func (h *elementHandler) Update(ctx context.Context, req *updateElementRequest) (*elementResponse, error) {
	id, command, err := toUpdateElementCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.elementService.Update(ctx, id, command)
	return toElementResponse(result), err
}

func (h *elementHandler) Delete(ctx context.Context, req *elementIDRequest) (*elementResponse, error) {
	id, err := toElementID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.elementService.Delete(ctx, id)
}
