package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// UserTraitRepository defines the user trait data access interface.
type UserTraitRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserTrait], error)
	Get(ctx context.Context, id int) (*entity.UserTrait, error)
	Create(ctx context.Context, e *entity.UserTrait) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// UserTraitService defines the user trait business logic interface.
type UserTraitService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserTraitResponse], error)
	Get(ctx context.Context, id int) (*dto.UserTraitResponse, error)
	Create(ctx context.Context, req *dto.CreateUserTraitRequest) (*dto.UserTraitResponse, error)
	Delete(ctx context.Context, id int) error
}
