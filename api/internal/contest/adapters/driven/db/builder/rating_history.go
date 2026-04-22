package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRatingHistory builds the create mutation for RatingHistory.
func BuildCreateRatingHistory(ctx context.Context, e *entity.RatingHistory) *generate.RatingHistoryCreate {
	return global.EntClient.DB(ctx).RatingHistory.Create().
		SetUserID(e.UserID).
		SetContestID(e.ContestID).
		SetOldRating(e.OldRating).
		SetNewRating(e.NewRating).
		SetRankPosition(e.RankPosition)
}
