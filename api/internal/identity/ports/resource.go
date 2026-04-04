package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ResourceRepository defines the resource data access interface.
type ResourceRepository interface {
	FindAll(ctx context.Context) ([]*entity.Resource, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Resource], error)
	Get(ctx context.Context, id int) (*entity.Resource, error)
	Create(ctx context.Context, e *entity.Resource) error
	Update(ctx context.Context, e *entity.Resource) error
	Delete(ctx context.Context, id int) error
	FindByIDs(ctx context.Context, ids []int) ([]*entity.Resource, error)
	Exists(ctx context.Context, id int) (bool, error)
}

// ResourceService defines the resource business logic interface.
type ResourceService interface {
	FindAll(ctx context.Context) ([]*dto.ResourceResponse, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ResourceResponse], error)
	Get(ctx context.Context, id int) (*dto.ResourceResponse, error)
	Create(ctx context.Context, req *dto.CreateResourceRequest) (*dto.ResourceResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateResourceRequest) (*dto.ResourceResponse, error)
	Delete(ctx context.Context, id int) error
}
