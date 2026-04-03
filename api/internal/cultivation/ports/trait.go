package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// TraitRepository defines the trait data access interface.
type TraitRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Trait], error)
	FindAll(ctx context.Context) ([]*entity.Trait, error)
	FindAllWithWeight(ctx context.Context) ([]*entity.TraitWithWeight, error)
	Get(ctx context.Context, id int) (*entity.Trait, error)
	Create(ctx context.Context, e *entity.Trait) error
	Update(ctx context.Context, e *entity.Trait) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// TraitService defines the trait business logic interface.
type TraitService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.TraitResponse], error)
	FindAll(ctx context.Context) ([]*dto.TraitResponse, error)
	Get(ctx context.Context, id int) (*dto.TraitResponse, error)
	Create(ctx context.Context, req *dto.CreateTraitRequest) (*dto.TraitResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateTraitRequest) (*dto.TraitResponse, error)
	Delete(ctx context.Context, id int) error
}
