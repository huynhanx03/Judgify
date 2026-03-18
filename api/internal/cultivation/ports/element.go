package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ElementRepository defines the element data access interface.
type ElementRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Element], error)
	Get(ctx context.Context, id int) (*entity.Element, error)
	Create(ctx context.Context, e *entity.Element) error
	Update(ctx context.Context, e *entity.Element) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// ElementService defines the element business logic interface.
type ElementService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ElementResponse], error)
	Get(ctx context.Context, id int) (*dto.ElementResponse, error)
	Create(ctx context.Context, req *dto.CreateElementRequest) (*dto.ElementResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateElementRequest) (*dto.ElementResponse, error)
	Delete(ctx context.Context, id int) error
}
