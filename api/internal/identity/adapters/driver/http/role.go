package http

import (
	"context"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/ports"
)

// RoleHandler defines the role HTTP handler interface.
type RoleHandler interface {
	FindAll(ctx context.Context, req *dto.FindAllRolesRequest) ([]*dto.RoleResponse, error)
	Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error)
	Get(ctx context.Context, req *dto.GetRoleRequest) (*dto.RoleResponse, error)
	Create(ctx context.Context, req *dto.CreateRoleRequest) (*dto.RoleResponse, error)
	Update(ctx context.Context, req *dto.UpdateRoleRequest) (*dto.RoleResponse, error)
	Delete(ctx context.Context, req *dto.DeleteRoleRequest) (*dto.RoleResponse, error)
}

type roleHandler struct {
	handler.BaseHandler
	roleService ports.RoleService
}

// NewRoleHandler creates a new RoleHandler instance.
func NewRoleHandler(roleService ports.RoleService) RoleHandler {
	return &roleHandler{
		roleService: roleService,
	}
}

// FindAll retrieves all roles without pagination.
func (h *roleHandler) FindAll(ctx context.Context, _ *dto.FindAllRolesRequest) ([]*dto.RoleResponse, error) {
	return h.roleService.FindAll(ctx)
}

// Find retrieves roles with pagination.
func (h *roleHandler) Find(ctx context.Context, req *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error) {
	return h.roleService.Find(ctx, req)
}

// Get retrieves a role by ID.
func (h *roleHandler) Get(ctx context.Context, req *dto.GetRoleRequest) (*dto.RoleResponse, error) {
	return h.roleService.Get(ctx, req.ID)
}

// Create creates a new role.
func (h *roleHandler) Create(ctx context.Context, req *dto.CreateRoleRequest) (*dto.RoleResponse, error) {
	return h.roleService.Create(ctx, req)
}

// Update updates an existing role.
func (h *roleHandler) Update(ctx context.Context, req *dto.UpdateRoleRequest) (*dto.RoleResponse, error) {
	return h.roleService.Update(ctx, req.ID, req)
}

// Delete removes a role by ID.
func (h *roleHandler) Delete(ctx context.Context, req *dto.DeleteRoleRequest) (*dto.RoleResponse, error) {
	return nil, h.roleService.Delete(ctx, req.ID)
}
