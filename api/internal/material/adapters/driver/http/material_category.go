package http

import (
	"context"

	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
)

// MaterialCategoryHandler defines the material category HTTP handler interface.
type MaterialCategoryHandler interface {
	FindAll(ctx context.Context) ([]*MaterialCategoryResponse, error)
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialCategoryResponse], error)
	Get(ctx context.Context, req *materialCategoryIDRequest) (*MaterialCategoryResponse, error)
	Create(ctx context.Context, req *CreateMaterialCategoryRequest) (*MaterialCategoryResponse, error)
	Update(ctx context.Context, req *UpdateMaterialCategoryRequest) (*MaterialCategoryResponse, error)
	Delete(ctx context.Context, req *materialCategoryIDRequest) (*MaterialCategoryResponse, error)
}

type materialCategoryHandler struct {
	categoryService ports.MaterialCategoryService
}

// NewMaterialCategoryHandler creates a new MaterialCategoryHandler.
func NewMaterialCategoryHandler(categoryService ports.MaterialCategoryService) MaterialCategoryHandler {
	return &materialCategoryHandler{categoryService: categoryService}
}

func (h *materialCategoryHandler) FindAll(ctx context.Context) ([]*MaterialCategoryResponse, error) {
	results, err := h.categoryService.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]*MaterialCategoryResponse, len(results))
	for index, result := range results {
		responses[index] = toMaterialCategoryResponse(result)
	}
	return responses, nil
}

func (h *materialCategoryHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*MaterialCategoryResponse], error) {
	results, err := h.categoryService.Find(ctx, req)
	if err != nil {
		return nil, err
	}
	responses := make([]*MaterialCategoryResponse, 0)
	if results.Records != nil {
		responses = make([]*MaterialCategoryResponse, len(*results.Records))
		for index, result := range *results.Records {
			responses[index] = toMaterialCategoryResponse(result)
		}
	}
	return &d.Paginated[*MaterialCategoryResponse]{Records: &responses, Pagination: results.Pagination}, nil
}

func (h *materialCategoryHandler) Get(ctx context.Context, req *materialCategoryIDRequest) (*MaterialCategoryResponse, error) {
	id, err := toMaterialCategoryID(req)
	if err != nil {
		return nil, err
	}
	result, err := h.categoryService.Get(ctx, id)
	return toMaterialCategoryResponse(result), err
}

func (h *materialCategoryHandler) Create(ctx context.Context, req *CreateMaterialCategoryRequest) (*MaterialCategoryResponse, error) {
	command := toCreateMaterialCategoryCommand(req)
	result, err := h.categoryService.Create(ctx, &command)
	return toMaterialCategoryResponse(result), err
}

func (h *materialCategoryHandler) Update(ctx context.Context, req *UpdateMaterialCategoryRequest) (*MaterialCategoryResponse, error) {
	id, command, err := toUpdateMaterialCategoryCommand(req)
	if err != nil {
		return nil, err
	}
	result, err := h.categoryService.Update(ctx, id, &command)
	return toMaterialCategoryResponse(result), err
}

func (h *materialCategoryHandler) Delete(ctx context.Context, req *materialCategoryIDRequest) (*MaterialCategoryResponse, error) {
	id, err := toMaterialCategoryID(req)
	if err != nil {
		return nil, err
	}
	if err := h.categoryService.Delete(ctx, id); err != nil {
		return nil, err
	}
	return nil, nil
}

// ensure handler.Wrap is available
var _ = handler.Wrap[any, any]
