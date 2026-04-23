package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// BuildCreateMaterialCategory builds the create mutation for MaterialCategory.
func BuildCreateMaterialCategory(ctx context.Context, e *entity.MaterialCategory) *generate.MaterialCategoryCreate {
	c := global.EntClient.DB(ctx).MaterialCategory.Create().
		SetName(e.Name)
	if e.Description != "" {
		c.SetDescription(e.Description)
	}
	return c
}

// BuildUpdateMaterialCategory builds the update mutation for MaterialCategory.
func BuildUpdateMaterialCategory(ctx context.Context, e *entity.MaterialCategory) *generate.MaterialCategoryUpdateOne {
	u := global.EntClient.DB(ctx).MaterialCategory.UpdateOneID(e.ID).
		SetName(e.Name)
	if e.Description != "" {
		u.SetDescription(e.Description)
	} else {
		u.ClearDescription()
	}
	return u
}
