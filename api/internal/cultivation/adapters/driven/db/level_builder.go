package db

import (
	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateLevel builds the create mutation for Level.
func BuildCreateLevel(client *generate.Client, e *entity.Level) *generate.LevelCreate {
	b := client.Level.Create().
		SetName(e.Name).
		SetMinExp(e.MinExp).
		SetDescription(e.Description)

	if e.ID != uuid.Nil {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateLevel builds the update mutation for Level.
func BuildUpdateLevel(client *generate.Client, e *entity.Level) *generate.LevelUpdateOne {
	return client.Level.UpdateOneID(e.ID).
		SetName(e.Name).
		SetMinExp(e.MinExp).
		SetDescription(e.Description)
}
