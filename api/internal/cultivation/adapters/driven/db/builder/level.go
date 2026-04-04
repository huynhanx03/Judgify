package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateLevel builds the create mutation for Level.
func BuildCreateLevel(ctx context.Context, e *entity.Level) *generate.LevelCreate {
	b := global.EntClient.DB(ctx).Level.Create().
		SetName(e.Name).
		SetMinExp(e.MinExp).
		SetDescription(e.Description)

	if e.ID != 0 {
		b.SetID(e.ID)
	}
	return b
}

// BuildUpdateLevel builds the update mutation for Level.
func BuildUpdateLevel(ctx context.Context, e *entity.Level) *generate.LevelUpdateOne {
	return global.EntClient.DB(ctx).Level.UpdateOneID(e.ID).
		SetName(e.Name).
		SetMinExp(e.MinExp).
		SetDescription(e.Description)
}
