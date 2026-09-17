package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/trait"
)

// BuildCreateTrait builds the create mutation for Trait.
func BuildCreateTrait(client *generate.Client, e *entity.Trait) *generate.TraitCreate {
	b := client.Trait.Create().
		SetCode(e.Code).
		SetType(trait.Type(e.Type)).
		SetName(e.Name).
		SetRarityID(e.RarityID).
		SetDescription(e.Description).
		SetDisplayOrder(e.DisplayOrder).
		SetActive(e.Active)
	if e.ID != uuid.Nil {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateTrait builds the update mutation for Trait.
func BuildUpdateTrait(client *generate.Client, e *entity.Trait, expectedVersion int64) *generate.TraitUpdateOne {
	b := client.Trait.UpdateOneID(e.ID).
		Where(trait.Version(expectedVersion)).
		SetType(trait.Type(e.Type)).
		SetName(e.Name).
		SetRarityID(e.RarityID).
		SetDescription(e.Description).
		SetDisplayOrder(e.DisplayOrder).
		SetActive(e.Active).
		AddVersion(1)
	return b
}
