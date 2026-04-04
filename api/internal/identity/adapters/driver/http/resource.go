package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

// ResourceHandler defines the resource HTTP handler interface.
type ResourceHandler interface {
	FindAll(ctx context.Context, req *dto.FindAllResourcesRequest) ([]*dto.ResourceResponse, error)
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ResourceResponse], error)
	Get(ctx context.Context, req *dto.GetResourceRequest) (*dto.ResourceResponse, error)
	Create(ctx context.Context, req *dto.CreateResourceRequest) (*dto.ResourceResponse, error)
	Update(ctx context.Context, req *dto.UpdateResourceRequest) (*dto.ResourceResponse, error)
	Delete(ctx context.Context, req *dto.DeleteResourceRequest) (*dto.ResourceResponse, error)
}

type resourceHandler struct {
	handler.BaseHandler
	resourceService ports.ResourceService
}

// NewResourceHandler creates a new ResourceHandler instance.
func NewResourceHandler(resourceService ports.ResourceService) ResourceHandler {
	return &resourceHandler{
		resourceService: resourceService,
	}
}

// FindAll retrieves all resources without pagination.
func (h *resourceHandler) FindAll(ctx context.Context, _ *dto.FindAllResourcesRequest) ([]*dto.ResourceResponse, error) {
	return h.resourceService.FindAll(ctx)
}

// Find retrieves resources with pagination.
func (h *resourceHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.ResourceResponse], error) {
	return h.resourceService.Find(ctx, req)
}

// Get retrieves a resource by ID.
func (h *resourceHandler) Get(ctx context.Context, req *dto.GetResourceRequest) (*dto.ResourceResponse, error) {
	return h.resourceService.Get(ctx, req.ID)
}

// Create creates a new resource.
func (h *resourceHandler) Create(ctx context.Context, req *dto.CreateResourceRequest) (*dto.ResourceResponse, error) {
	return h.resourceService.Create(ctx, req)
}

// Update updates an existing resource.
func (h *resourceHandler) Update(ctx context.Context, req *dto.UpdateResourceRequest) (*dto.ResourceResponse, error) {
	return h.resourceService.Update(ctx, req.ID, req)
}

// Delete removes a resource by ID.
func (h *resourceHandler) Delete(ctx context.Context, req *dto.DeleteResourceRequest) (*dto.ResourceResponse, error) {
	return nil, h.resourceService.Delete(ctx, req.ID)
}
