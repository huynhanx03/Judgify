package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// BuildCreateDifficulty builds the create mutation for Difficulty entity.
func BuildCreateDifficulty(ctx context.Context, e *entity.Difficulty) *generate.DifficultyCreate {
	create := global.EntClient.DB(ctx).Difficulty.Create().
		SetName(e.Name).
		SetLevel(e.Level).
		SetExpReward(e.ExpReward)
	if e.Description != "" {
		create.SetDescription(e.Description)
	}
	return create
}

// BuildUpdateDifficulty builds the update mutation for Difficulty entity.
func BuildUpdateDifficulty(ctx context.Context, e *entity.Difficulty) *generate.DifficultyUpdateOne {
	return global.EntClient.DB(ctx).Difficulty.UpdateOneID(e.ID).
		SetName(e.Name).
		SetLevel(e.Level).
		SetExpReward(e.ExpReward).
		SetDescription(e.Description)
}
