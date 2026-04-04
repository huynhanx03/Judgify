package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRarity builds the create mutation for Rarity.
func BuildCreateRarity(ctx context.Context, r *entity.Rarity) *generate.RarityCreate {
	b := global.EntClient.DB(ctx).Rarity.Create().
		SetName(r.Name).
		SetCode(r.Code).
		SetWeight(r.Weight).
		SetDescription(r.Description)

	if r.ID != 0 {
		b.SetID(r.ID)
	}
	return b
}

// BuildUpdateRarity builds the update mutation for Rarity.
func BuildUpdateRarity(ctx context.Context, r *entity.Rarity) *generate.RarityUpdateOne {
	return global.EntClient.DB(ctx).Rarity.UpdateOneID(r.ID).
		SetName(r.Name).
		SetCode(r.Code).
		SetWeight(r.Weight).
		SetDescription(r.Description)
}
