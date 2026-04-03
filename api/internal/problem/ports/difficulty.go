package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// DifficultyRepository defines the difficulty data access interface.
type DifficultyRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Difficulty], error)
	FindAll(ctx context.Context) ([]*entity.Difficulty, error)
	Get(ctx context.Context, id int) (*entity.Difficulty, error)
	Create(ctx context.Context, e *entity.Difficulty) error
	Update(ctx context.Context, e *entity.Difficulty) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// DifficultyService defines the difficulty business logic interface.
type DifficultyService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.DifficultyResponse], error)
	FindAll(ctx context.Context) ([]*dto.DifficultyResponse, error)
	Get(ctx context.Context, id int) (*dto.DifficultyResponse, error)
	Create(ctx context.Context, req *dto.CreateDifficultyRequest) (*dto.DifficultyResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateDifficultyRequest) (*dto.DifficultyResponse, error)
	Delete(ctx context.Context, id int) error
}
