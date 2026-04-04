package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateUserStats builds the create mutation for UserStats.
func BuildCreateUserStats(ctx context.Context, e *entity.UserStats) *generate.UserStatsCreate {
	b := global.EntClient.DB(ctx).UserStats.Create().
		SetUserID(e.UserID).
		SetTotalExp(e.TotalExp).
		SetRating(e.Rating)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateUserStats builds the update mutation for UserStats.
func BuildUpdateUserStats(ctx context.Context, e *entity.UserStats) *generate.UserStatsUpdateOne {
	return global.EntClient.DB(ctx).UserStats.UpdateOneID(e.ID).
		SetTotalExp(e.TotalExp).
		SetRating(e.Rating)
}
