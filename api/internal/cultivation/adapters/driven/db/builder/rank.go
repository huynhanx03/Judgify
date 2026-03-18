package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRank builds the create mutation for Rank.
func BuildCreateRank(ctx context.Context, e *entity.Rank) *generate.RankCreate {
	b := global.EntClient.DB(ctx).Rank.Create().
		SetName(e.Name).
		SetMinRating(e.MinRating).
		SetDescription(e.Description)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateRank builds the update mutation for Rank.
func BuildUpdateRank(ctx context.Context, e *entity.Rank) *generate.RankUpdateOne {
	return global.EntClient.DB(ctx).Rank.UpdateOneID(e.ID).
		SetName(e.Name).
		SetMinRating(e.MinRating).
		SetDescription(e.Description)
}
