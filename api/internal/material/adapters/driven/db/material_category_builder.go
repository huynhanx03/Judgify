package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// BuildCreateMaterialCategory builds the create mutation for MaterialCategory.
func BuildCreateMaterialCategory(client *generate.Client, e *entity.MaterialCategory) *generate.MaterialCategoryCreate {
	c := client.MaterialCategory.Create().
		SetName(e.Name)
	if e.Description != "" {
		c.SetDescription(e.Description)
	}
	return c
}

// BuildUpdateMaterialCategory builds the update mutation for MaterialCategory.
func BuildUpdateMaterialCategory(client *generate.Client, e *entity.MaterialCategory) *generate.MaterialCategoryUpdateOne {
	u := client.MaterialCategory.UpdateOneID(e.ID).
		SetName(e.Name)
	if e.Description != "" {
		u.SetDescription(e.Description)
	} else {
		u.ClearDescription()
	}
	return u
}
