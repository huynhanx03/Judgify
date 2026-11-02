package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRarity builds the create mutation for Rarity.
func BuildCreateRarity(client *generate.Client, r *entity.Rarity) *generate.RarityCreate {
	b := client.Rarity.Create().
		SetName(r.Name).
		SetCode(r.Code).
		SetWeight(r.Weight).
		SetDescription(r.Description)

	if r.ID != uuid.Nil {
		b.SetID(r.ID)
	}
	return b
}

// BuildUpdateRarity builds the update mutation for Rarity.
func BuildUpdateRarity(client *generate.Client, r *entity.Rarity) *generate.RarityUpdateOne {
	return client.Rarity.UpdateOneID(r.ID).
		SetName(r.Name).
		SetCode(r.Code).
		SetWeight(r.Weight).
		SetDescription(r.Description)
}
