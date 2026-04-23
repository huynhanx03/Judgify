package ports

import (
	"context"

	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
)

// RankingService defines the ranking business logic interface.
type RankingService interface {
	GetTopByRating(ctx context.Context, limit int) ([]*dto.RankingEntry, error)
	GetTopByExp(ctx context.Context, limit int) ([]*dto.RankingEntry, error)
}
