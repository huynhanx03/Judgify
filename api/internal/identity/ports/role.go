package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// RoleRepository defines the role data access interface.
type RoleRepository interface {
	FindAll(ctx context.Context) ([]*entity.Role, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Role], error)
	Get(ctx context.Context, id int) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	Create(ctx context.Context, e *entity.Role) error
	Update(ctx context.Context, e *entity.Role) error
	UpdateBulk(ctx context.Context, entities []*entity.Role) error
	Delete(ctx context.Context, id int) error
	FindDescendants(ctx context.Context, lft, rgt int) ([]*entity.Role, error)
	Exists(ctx context.Context, id int) (bool, error)
}

// RoleService defines the role business logic interface.
type RoleService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RoleResponse], error)
	Get(ctx context.Context, id int) (*dto.RoleResponse, error)
	Create(ctx context.Context, req *dto.CreateRoleRequest) (*dto.RoleResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateRoleRequest) (*dto.RoleResponse, error)
	Delete(ctx context.Context, id int) error
}
