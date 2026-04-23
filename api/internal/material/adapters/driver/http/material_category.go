package http

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/ports"
)

// MaterialCategoryHandler defines the material category HTTP handler interface.
type MaterialCategoryHandler interface {
	FindAll(ctx context.Context, req *dto.FindAllMaterialCategoriesRequest) ([]*dto.MaterialCategoryResponse, error)
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.MaterialCategoryResponse], error)
	Get(ctx context.Context, req *dto.GetMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
	Create(ctx context.Context, req *dto.CreateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
	Update(ctx context.Context, req *dto.UpdateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
	Delete(ctx context.Context, req *dto.DeleteMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
}

type materialCategoryHandler struct {
	categoryService ports.MaterialCategoryService
}

// NewMaterialCategoryHandler creates a new MaterialCategoryHandler.
func NewMaterialCategoryHandler(categoryService ports.MaterialCategoryService) MaterialCategoryHandler {
	return &materialCategoryHandler{categoryService: categoryService}
}

func (h *materialCategoryHandler) FindAll(ctx context.Context, _ *dto.FindAllMaterialCategoriesRequest) ([]*dto.MaterialCategoryResponse, error) {
	return h.categoryService.FindAll(ctx)
}

func (h *materialCategoryHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.MaterialCategoryResponse], error) {
	return h.categoryService.Find(ctx, req)
}

func (h *materialCategoryHandler) Get(ctx context.Context, req *dto.GetMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	return h.categoryService.Get(ctx, req.ID)
}

func (h *materialCategoryHandler) Create(ctx context.Context, req *dto.CreateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	return h.categoryService.Create(ctx, req)
}

func (h *materialCategoryHandler) Update(ctx context.Context, req *dto.UpdateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	return h.categoryService.Update(ctx, req.ID, req)
}

func (h *materialCategoryHandler) Delete(ctx context.Context, req *dto.DeleteMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error) {
	if err := h.categoryService.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ensure handler.Wrap is available
var _ = handler.Wrap[any, any]
