package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateRank builds the create mutation for Rank.
func BuildCreateRank(client *generate.Client, e *entity.Rank) *generate.RankCreate {
	b := client.Rank.Create().
		SetName(e.Name).
		SetMinRating(e.MinRating).
		SetDescription(e.Description)

	if e.ID != uuid.Nil {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateRank builds the update mutation for Rank.
func BuildUpdateRank(client *generate.Client, e *entity.Rank) *generate.RankUpdateOne {
	return client.Rank.UpdateOneID(e.ID).
		SetName(e.Name).
		SetMinRating(e.MinRating).
		SetDescription(e.Description)
}
