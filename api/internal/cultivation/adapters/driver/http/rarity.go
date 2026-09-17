package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// RarityHandler defines the rarity HTTP handler interface.
type RarityHandler interface {
	Find(context.Context, *d.QueryOptions) (*d.Paginated[*rarityResponse], error)
	FindAll(context.Context) ([]*rarityResponse, error)
	Get(context.Context, *rarityIDRequest) (*rarityResponse, error)
	Create(context.Context, *createRarityRequest) (*rarityResponse, error)
	Update(context.Context, *updateRarityRequest) (*rarityResponse, error)
	Delete(context.Context, *rarityIDRequest) (*rarityResponse, error)
}

type rarityHandler struct {
	handler.BaseHandler
	rarityService ports.RarityService
}

func NewRarityHandler(svc ports.RarityService) RarityHandler {
	return &rarityHandler{rarityService: svc}
}

func (h *rarityHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*rarityResponse], error) {
	result, err := h.rarityService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	return toRarityPage(result), nil
}

func (h *rarityHandler) FindAll(ctx context.Context) ([]*rarityResponse, error) {
	result, err := h.rarityService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return toRarityResponses(result), nil
}

func (h *rarityHandler) Get(ctx context.Context, req *rarityIDRequest) (*rarityResponse, error) {
	id, err := toRarityID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.rarityService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRarityResponse(result), nil
}

func (h *rarityHandler) Create(ctx context.Context, req *createRarityRequest) (*rarityResponse, error) {
	result, err := h.rarityService.Create(ctx, toCreateRarityCommand(req))
	if err != nil {
		return nil, err
	}
	return toRarityResponse(result), nil
}

func (h *rarityHandler) Update(ctx context.Context, req *updateRarityRequest) (*rarityResponse, error) {
	id, command, err := toUpdateRarityCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.rarityService.Update(ctx, id, command)
	if err != nil {
		return nil, err
	}
	return toRarityResponse(result), nil
}

func (h *rarityHandler) Delete(ctx context.Context, req *rarityIDRequest) (*rarityResponse, error) {
	id, err := toRarityID(req)
	if err != nil {
		return nil, err
	}
	return nil, h.rarityService.Delete(ctx, id)
}
