package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// TagRepository defines the tag data access interface.
type TagRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Tag], error)
	FindAll(ctx context.Context) ([]*entity.Tag, error)
	Get(ctx context.Context, id int) (*entity.Tag, error)
	Create(ctx context.Context, e *entity.Tag) error
	Update(ctx context.Context, e *entity.Tag) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	FindByIDs(ctx context.Context, ids []int) ([]*entity.Tag, error)
}

// TagService defines the tag business logic interface.
type TagService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.TagResponse], error)
	FindAll(ctx context.Context) ([]*dto.TagResponse, error)
	Get(ctx context.Context, id int) (*dto.TagResponse, error)
	Create(ctx context.Context, req *dto.CreateTagRequest) (*dto.TagResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateTagRequest) (*dto.TagResponse, error)
	Delete(ctx context.Context, id int) error
}
