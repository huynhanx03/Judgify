package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// RarityRepository defines the rarity data access interface.
type RarityRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rarity], error)
	FindAll(ctx context.Context) ([]*entity.Rarity, error)
	Get(ctx context.Context, id int) (*entity.Rarity, error)
	Create(ctx context.Context, r *entity.Rarity) error
	Update(ctx context.Context, r *entity.Rarity) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// RarityService defines the rarity business logic interface.
type RarityService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RarityResponse], error)
	FindAll(ctx context.Context) ([]*dto.RarityResponse, error)
	Get(ctx context.Context, id int) (*dto.RarityResponse, error)
	Create(ctx context.Context, req *dto.CreateRarityRequest) (*dto.RarityResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateRarityRequest) (*dto.RarityResponse, error)
	Delete(ctx context.Context, id int) error
}
