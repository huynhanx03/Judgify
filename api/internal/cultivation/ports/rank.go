package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// RankRepository defines the rank data access interface.
type RankRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Rank], error)
	FindAll(ctx context.Context) ([]*entity.Rank, error)
	Get(ctx context.Context, id int) (*entity.Rank, error)
	Create(ctx context.Context, e *entity.Rank) error
	Update(ctx context.Context, e *entity.Rank) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// RankService defines the rank business logic interface.
type RankService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.RankResponse], error)
	FindAll(ctx context.Context) ([]*dto.RankResponse, error)
	Get(ctx context.Context, id int) (*dto.RankResponse, error)
	Create(ctx context.Context, req *dto.CreateRankRequest) (*dto.RankResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateRankRequest) (*dto.RankResponse, error)
	Delete(ctx context.Context, id int) error
}
