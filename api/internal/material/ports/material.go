package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// MaterialCategoryRepository defines the material category data access interface.
type MaterialCategoryRepository interface {
	FindAll(ctx context.Context) ([]*entity.MaterialCategory, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.MaterialCategory], error)
	Get(ctx context.Context, id int) (*entity.MaterialCategory, error)
	Create(ctx context.Context, e *entity.MaterialCategory) error
	Update(ctx context.Context, e *entity.MaterialCategory) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
}

// MaterialRepository defines the material data access interface.
type MaterialRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Material], error)
	Get(ctx context.Context, id int) (*entity.Material, error)
	Create(ctx context.Context, e *entity.Material) error
	Update(ctx context.Context, e *entity.Material) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	IncrementViewCount(ctx context.Context, id int) error
	AddTags(ctx context.Context, materialID int, tagIDs []int) error
	RemoveAllTags(ctx context.Context, materialID int) error
}

// MaterialCategoryService defines the material category business logic interface.
type MaterialCategoryService interface {
	FindAll(ctx context.Context) ([]*dto.MaterialCategoryResponse, error)
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialCategoryResponse], error)
	Get(ctx context.Context, id int) (*dto.MaterialCategoryResponse, error)
	Create(ctx context.Context, req *dto.CreateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateMaterialCategoryRequest) (*dto.MaterialCategoryResponse, error)
	Delete(ctx context.Context, id int) error
}

// MaterialService defines the material business logic interface.
type MaterialService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.MaterialResponse], error)
	Get(ctx context.Context, id int) (*dto.MaterialResponse, error)
	Create(ctx context.Context, authorID int, req *dto.CreateMaterialRequest) (*dto.MaterialResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateMaterialRequest) (*dto.MaterialResponse, error)
	Delete(ctx context.Context, id int) error
}
