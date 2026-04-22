package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// ContestRepository defines the contest data access interface.
type ContestRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.Contest], error)
	Get(ctx context.Context, id int) (*entity.Contest, error)
	Create(ctx context.Context, e *entity.Contest) error
	Update(ctx context.Context, e *entity.Contest) error
	Delete(ctx context.Context, id int) error
	AddProblems(ctx context.Context, contestID int, problemIDs []int) error
	RemoveAllProblems(ctx context.Context, contestID int) error
	GetProblemIDs(ctx context.Context, contestID int) ([]int, error)
	FindIDsByStatus(ctx context.Context, status string) ([]int, error)
	UpdateStatus(ctx context.Context, id int, status string) error
	CountRegistrations(ctx context.Context, contestID int) (int, error)
}

// ContestService defines the contest business logic interface.
type ContestService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.ContestResponse], error)
	Get(ctx context.Context, id int, userID int) (*dto.ContestResponse, error)
	Create(ctx context.Context, authorID int, req *dto.CreateContestRequest) (*dto.ContestResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateContestRequest) (*dto.ContestResponse, error)
	Delete(ctx context.Context, id int) error
}
