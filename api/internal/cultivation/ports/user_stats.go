package ports

import (
	"context"

	d "github.com/huynhanx03/judgify/pkg/dto"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// UserStatsRepository defines the user stats data access interface.
type UserStatsRepository interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*entity.UserStats], error)
	Get(ctx context.Context, id int) (*entity.UserStats, error)
	GetByUserID(ctx context.Context, userID int) (*entity.UserStats, error)
	Create(ctx context.Context, e *entity.UserStats) error
	Update(ctx context.Context, e *entity.UserStats) error
	Delete(ctx context.Context, id int) error
	Exists(ctx context.Context, id int) (bool, error)
	AddExp(ctx context.Context, userID int, exp int64) error
	IncrementSubmission(ctx context.Context, userID int, accepted bool) error
	GetTopSortedWithUser(ctx context.Context, sortField string, limit int) ([]*entity.UserStatsWithUser, error)
}

// UserDifficultyStatsRepository defines the difficulty stats data access interface.
type UserDifficultyStatsRepository interface {
	GetByUserID(ctx context.Context, userID int) ([]*entity.UserDifficultyStats, error)
	CreateBulk(ctx context.Context, userID int, difficultyIDs []int) error
	IncrementSolved(ctx context.Context, userID, difficultyID int) error
}

// UserTagStatsRepository defines the tag stats data access interface.
type UserTagStatsRepository interface {
	GetByUserID(ctx context.Context, userID int) ([]*entity.UserTagStats, error)
	IncrementSolved(ctx context.Context, userID, tagID int) error
}

// UserStatsService defines the user stats business logic interface.
type UserStatsService interface {
	Find(ctx context.Context, opts *d.QueryOptions) (*d.Paginated[*dto.UserStatsResponse], error)
	Get(ctx context.Context, id int) (*dto.UserStatsResponse, error)
	Create(ctx context.Context, req *dto.CreateUserStatsRequest) (*dto.UserStatsResponse, error)
	Update(ctx context.Context, id int, req *dto.UpdateUserStatsRequest) (*dto.UserStatsResponse, error)
	Delete(ctx context.Context, id int) error
}
