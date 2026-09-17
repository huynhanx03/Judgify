package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/material"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// BuildCreateMaterial builds the create mutation for Material.
func BuildCreateMaterial(client *generate.Client, e *entity.Material) *generate.MaterialCreate {
	c := client.Material.Create().
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
func BuildUpdateMaterial(client *generate.Client, e *entity.Material) *generate.MaterialUpdateOne {
	u := client.Material.UpdateOneID(e.ID).
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
