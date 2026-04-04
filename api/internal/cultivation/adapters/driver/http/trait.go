package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/ports"
)

// TraitHandler defines the trait HTTP handler interface.
type TraitHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.TraitResponse], error)
	FindAll(ctx context.Context, req *dto.FindAllTraitsRequest) ([]*dto.TraitResponse, error)
	Get(ctx context.Context, req *dto.GetTraitRequest) (*dto.TraitResponse, error)
	Create(ctx context.Context, req *dto.CreateTraitRequest) (*dto.TraitResponse, error)
	Update(ctx context.Context, req *dto.UpdateTraitRequest) (*dto.TraitResponse, error)
	Delete(ctx context.Context, req *dto.DeleteTraitRequest) (*dto.TraitResponse, error)
}

type traitHandler struct {
	handler.BaseHandler
	traitService ports.TraitService
}

func NewTraitHandler(svc ports.TraitService) TraitHandler {
	return &traitHandler{traitService: svc}
}

func (h *traitHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.TraitResponse], error) {
	return h.traitService.Find(ctx, req)
}

func (h *traitHandler) FindAll(ctx context.Context, _ *dto.FindAllTraitsRequest) ([]*dto.TraitResponse, error) {
	return h.traitService.FindAll(ctx)
}

func (h *traitHandler) Get(ctx context.Context, req *dto.GetTraitRequest) (*dto.TraitResponse, error) {
	return h.traitService.Get(ctx, req.ID)
}

func (h *traitHandler) Create(ctx context.Context, req *dto.CreateTraitRequest) (*dto.TraitResponse, error) {
	return h.traitService.Create(ctx, req)
}

func (h *traitHandler) Update(ctx context.Context, req *dto.UpdateTraitRequest) (*dto.TraitResponse, error) {
	return h.traitService.Update(ctx, req.ID, req)
}

func (h *traitHandler) Delete(ctx context.Context, req *dto.DeleteTraitRequest) (*dto.TraitResponse, error) {
	return nil, h.traitService.Delete(ctx, req.ID)
}
