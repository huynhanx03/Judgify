package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ProblemRepository defines the problem data access interface.
type ProblemRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Problem], error)
	Get(ctx context.Context, id int) (*entity.Problem, error)
	Create(ctx context.Context, e *entity.Problem) error
	Update(ctx context.Context, e *entity.Problem) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	AddTags(ctx context.Context, problemID int, tagIDs []int) error
	RemoveTags(ctx context.Context, problemID int, tagIDs []int) error
	ReplaceTags(ctx context.Context, problemID int, tagIDs []int) error
	GetTagIDs(ctx context.Context, problemID int) ([]int, error)
}

// ProblemService defines the problem business logic interface.
type ProblemService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ProblemResponse], error)
	Get(ctx context.Context, id int) (*dto.ProblemResponse, error)
	Create(ctx context.Context, authorID int, req *dto.CreateProblemRequest) (*dto.ProblemResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateProblemRequest) (*dto.ProblemResponse, error)
	Delete(ctx context.Context, id int) error
}
