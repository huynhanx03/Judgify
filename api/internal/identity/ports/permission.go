package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// PermissionRepository defines the permission data access interface.
type PermissionRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Permission], error)
	Get(ctx context.Context, id int) (*entity.Permission, error)
	Create(ctx context.Context, e *entity.Permission) error
	Update(ctx context.Context, e *entity.Permission) error
	Delete(ctx context.Context, id int) error
	FindByRoleIDs(ctx context.Context, roleIDs []int) ([]*entity.Permission, error)
	Exists(ctx context.Context, id int) (bool, error)
}

// PermissionService defines the permission business logic interface.
type PermissionService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.PermissionResponse], error)
	Get(ctx context.Context, id int) (*dto.PermissionResponse, error)
	Create(ctx context.Context, req *dto.CreatePermissionRequest) (*dto.PermissionResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdatePermissionRequest) (*dto.PermissionResponse, error)
	Delete(ctx context.Context, id int) error
}
