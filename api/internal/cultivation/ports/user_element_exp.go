package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// UserElementExpRepository defines the user element EXP data access interface.
type UserElementExpRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserElementExp], error)
	Get(ctx context.Context, id int) (*entity.UserElementExp, error)
	Create(ctx context.Context, e *entity.UserElementExp) error
	Update(ctx context.Context, e *entity.UserElementExp) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// UserElementExpService defines the user element EXP business logic interface.
type UserElementExpService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserElementExpResponse], error)
	Get(ctx context.Context, id int) (*dto.UserElementExpResponse, error)
	Create(ctx context.Context, req *dto.CreateUserElementExpRequest) (*dto.UserElementExpResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateUserElementExpRequest) (*dto.UserElementExpResponse, error)
	Delete(ctx context.Context, id int) error
}
