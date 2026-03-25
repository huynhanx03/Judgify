package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// LevelRepository defines the level data access interface.
type LevelRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Level], error)
	Get(ctx context.Context, id int) (*entity.Level, error)
	Create(ctx context.Context, e *entity.Level) error
	Update(ctx context.Context, e *entity.Level) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// LevelService defines the level business logic interface.
type LevelService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.LevelResponse], error)
	Get(ctx context.Context, id int) (*dto.LevelResponse, error)
	Create(ctx context.Context, req *dto.CreateLevelRequest) (*dto.LevelResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateLevelRequest) (*dto.LevelResponse, error)
	Delete(ctx context.Context, id int) error
}
