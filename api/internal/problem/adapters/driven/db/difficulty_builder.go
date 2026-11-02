package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// BuildCreateDifficulty builds the create mutation for Difficulty entity.
func BuildCreateDifficulty(client *generate.Client, e *entity.Difficulty) *generate.DifficultyCreate {
	create := client.Difficulty.Create().
		SetName(e.Name).
		SetLevel(e.Level).
		SetExpReward(e.ExpReward).
		SetVersion(1)
	if e.Description != "" {
		create.SetDescription(e.Description)
	}
	return create
}

// BuildUpdateDifficulty builds the update mutation for Difficulty entity.
func BuildUpdateDifficulty(client *generate.Client, e *entity.Difficulty, nextVersion int64) *generate.DifficultyUpdateOne {
	return client.Difficulty.UpdateOneID(e.ID).
		SetName(e.Name).
		SetLevel(e.Level).
		SetExpReward(e.ExpReward).
		SetDescription(e.Description).
		SetVersion(nextVersion)
}
