package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/material"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// BuildCreateMaterial builds the create mutation for Material.
func BuildCreateMaterial(ctx context.Context, e *entity.Material) *generate.MaterialCreate {
	c := global.EntClient.DB(ctx).Material.Create().
		SetTitle(e.Title).
		SetDifficultyID(e.DifficultyID).
		SetAuthorID(e.AuthorID).
		SetCategoryID(e.CategoryID).
		SetStatus(material.Status(e.Status)).
		SetVisibility(material.Visibility(e.Visibility)).
		SetViewCount(e.ViewCount).
		SetEstimatedReadTime(e.EstimatedReadTime)

	if e.Description != "" {
		c.SetDescription(e.Description)
	}
	if e.Content != "" {
		c.SetContent(e.Content)
	}
	if e.GroupID != nil {
		c.SetGroupID(*e.GroupID)
	}
	return c
}

// BuildUpdateMaterial builds the update mutation for Material.
func BuildUpdateMaterial(ctx context.Context, e *entity.Material) *generate.MaterialUpdateOne {
	u := global.EntClient.DB(ctx).Material.UpdateOneID(e.ID).
		SetTitle(e.Title).
		SetDifficultyID(e.DifficultyID).
		SetCategoryID(e.CategoryID).
		SetStatus(material.Status(e.Status)).
		SetVisibility(material.Visibility(e.Visibility)).
		SetEstimatedReadTime(e.EstimatedReadTime)

	if e.Description != "" {
		u.SetDescription(e.Description)
	} else {
		u.ClearDescription()
	}
	if e.Content != "" {
		u.SetContent(e.Content)
	} else {
		u.ClearContent()
	}
	return u
}
