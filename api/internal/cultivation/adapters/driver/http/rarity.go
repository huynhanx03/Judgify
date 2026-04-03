package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// RarityHandler defines the rarity HTTP handler interface.
type RarityHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RarityResponse], error)
	FindAll(ctx context.Context, req *dto.FindAllRaritiesRequest) ([]*dto.RarityResponse, error)
	Get(ctx context.Context, req *dto.GetRarityRequest) (*dto.RarityResponse, error)
	Create(ctx context.Context, req *dto.CreateRarityRequest) (*dto.RarityResponse, error)
	Update(ctx context.Context, req *dto.UpdateRarityRequest) (*dto.RarityResponse, error)
	Delete(ctx context.Context, req *dto.DeleteRarityRequest) (*dto.RarityResponse, error)
}

type rarityHandler struct {
	handler.BaseHandler
	rarityService ports.RarityService
}

func NewRarityHandler(svc ports.RarityService) RarityHandler {
	return &rarityHandler{rarityService: svc}
}

func (h *rarityHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RarityResponse], error) {
	return h.rarityService.Find(ctx, req)
}

func (h *rarityHandler) FindAll(ctx context.Context, _ *dto.FindAllRaritiesRequest) ([]*dto.RarityResponse, error) {
	return h.rarityService.FindAll(ctx)
}

func (h *rarityHandler) Get(ctx context.Context, req *dto.GetRarityRequest) (*dto.RarityResponse, error) {
	return h.rarityService.Get(ctx, req.ID)
}

func (h *rarityHandler) Create(ctx context.Context, req *dto.CreateRarityRequest) (*dto.RarityResponse, error) {
	return h.rarityService.Create(ctx, req)
}

func (h *rarityHandler) Update(ctx context.Context, req *dto.UpdateRarityRequest) (*dto.RarityResponse, error) {
	return h.rarityService.Update(ctx, req.ID, req)
}

func (h *rarityHandler) Delete(ctx context.Context, req *dto.DeleteRarityRequest) (*dto.RarityResponse, error) {
	return nil, h.rarityService.Delete(ctx, req.ID)
}
