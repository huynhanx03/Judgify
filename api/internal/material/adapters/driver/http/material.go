package http

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	"github.com/huynhanx03/judgify/pkg/constraints"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

// MaterialHandler defines the material HTTP handler interface.
type MaterialHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.MaterialResponse], error)
	Get(ctx context.Context, req *dto.GetMaterialRequest) (*dto.MaterialResponse, error)
	Create(ctx context.Context, req *dto.CreateMaterialRequest) (*dto.MaterialResponse, error)
	Update(ctx context.Context, req *dto.UpdateMaterialRequest) (*dto.MaterialResponse, error)
	Delete(ctx context.Context, req *dto.DeleteMaterialRequest) (*dto.MaterialResponse, error)
}

type materialHandler struct {
	materialService ports.MaterialService
}

// NewMaterialHandler creates a new MaterialHandler.
func NewMaterialHandler(materialService ports.MaterialService) MaterialHandler {
	return &materialHandler{materialService: materialService}
}

func (h *materialHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.MaterialResponse], error) {
	return h.materialService.Find(ctx, req)
}

func (h *materialHandler) Get(ctx context.Context, req *dto.GetMaterialRequest) (*dto.MaterialResponse, error) {
	return h.materialService.Get(ctx, req.ID)
}

func (h *materialHandler) Create(ctx context.Context, req *dto.CreateMaterialRequest) (*dto.MaterialResponse, error) {
	userIDVal := ctx.Value(constraints.ContextKeyUserID)
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, apperr.New(response.CodeUnauthorized, "unauthorized", nil)
	}
	return h.materialService.Create(ctx, userID, req)
}

func (h *materialHandler) Update(ctx context.Context, req *dto.UpdateMaterialRequest) (*dto.MaterialResponse, error) {
	return h.materialService.Update(ctx, req.ID, req)
}

func (h *materialHandler) Delete(ctx context.Context, req *dto.DeleteMaterialRequest) (*dto.MaterialResponse, error) {
	if err := h.materialService.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return nil, nil
}
