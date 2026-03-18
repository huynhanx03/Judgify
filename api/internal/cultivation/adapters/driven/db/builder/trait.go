package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/trait"
)

// BuildCreateTrait builds the create mutation for Trait.
func BuildCreateTrait(ctx context.Context, e *entity.Trait) *generate.TraitCreate {
	b := global.EntClient.DB(ctx).Trait.Create().
		SetType(trait.Type(e.Type)).
		SetName(e.Name).
		SetRarity(trait.Rarity(e.Rarity)).
		SetWeight(e.Weight).
		SetDescription(e.Description)

	if e.Metadata != nil {
		b.SetMetadata(e.Metadata)
	}
	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateTrait builds the update mutation for Trait.
func BuildUpdateTrait(ctx context.Context, e *entity.Trait) *generate.TraitUpdateOne {
	b := global.EntClient.DB(ctx).Trait.UpdateOneID(e.ID).
		SetType(trait.Type(e.Type)).
		SetName(e.Name).
		SetRarity(trait.Rarity(e.Rarity)).
		SetWeight(e.Weight).
		SetDescription(e.Description)

	if e.Metadata != nil {
		b.SetMetadata(e.Metadata)
	}
	return b
}
