package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// RatingHistoryRepository defines the rating history data access interface.
type RatingHistoryRepository interface {
	CreateBulk(ctx context.Context, records []*entity.RatingHistory) error
	FindByContest(ctx context.Context, contestID int) ([]*entity.RatingHistory, error)
	CountByUser(ctx context.Context, userID int) (int, error)
	CountByUsers(ctx context.Context, userIDs []int) (map[int]int, error)
}

// RatingService defines the rating business logic interface.
type RatingService interface {
	CalculateRating(ctx context.Context, contestID int) error
	GetContestRatingChanges(ctx context.Context, contestID int) ([]*dto.RatingChangeResponse, error)
}
