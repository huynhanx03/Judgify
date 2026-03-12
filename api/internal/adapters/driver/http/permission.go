package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/core/dto"
	"github.com/huynhanx03/judgify/internal/ports"
)

// PermissionHandler defines the permission HTTP handler interface.
type PermissionHandler interface {
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.PermissionResponse], error)
	Get(ctx context.Context, req *dto.GetPermissionRequest) (*dto.PermissionResponse, error)
	Create(ctx context.Context, req *dto.CreatePermissionRequest) (*dto.PermissionResponse, error)
	Update(ctx context.Context, req *dto.UpdatePermissionRequest) (*dto.PermissionResponse, error)
	Delete(ctx context.Context, req *dto.DeletePermissionRequest) (*dto.PermissionResponse, error)
}

type permissionHandler struct {
	handler.BaseHandler
	permissionService ports.PermissionService
}

// NewPermissionHandler creates a new PermissionHandler instance.
func NewPermissionHandler(permissionService ports.PermissionService) PermissionHandler {
	return &permissionHandler{
		permissionService: permissionService,
	}
}

// Find retrieves permissions with pagination.
func (h *permissionHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.PermissionResponse], error) {
	return h.permissionService.Find(ctx, req)
}

// Get retrieves a permission by ID.
func (h *permissionHandler) Get(ctx context.Context, req *dto.GetPermissionRequest) (*dto.PermissionResponse, error) {
	return h.permissionService.Get(ctx, req.ID)
}

// Create creates a new permission.
func (h *permissionHandler) Create(ctx context.Context, req *dto.CreatePermissionRequest) (*dto.PermissionResponse, error) {
	return h.permissionService.Create(ctx, req)
}

// Update updates an existing permission.
func (h *permissionHandler) Update(ctx context.Context, req *dto.UpdatePermissionRequest) (*dto.PermissionResponse, error) {
	return h.permissionService.Update(ctx, req.ID, req)
}

// Delete removes a permission by ID.
func (h *permissionHandler) Delete(ctx context.Context, req *dto.DeletePermissionRequest) (*dto.PermissionResponse, error) {
	return nil, h.permissionService.Delete(ctx, req.ID)
}
