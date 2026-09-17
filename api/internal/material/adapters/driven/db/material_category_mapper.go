package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// ToMaterialCategoryEntity converts Ent MaterialCategory model to domain entity.
func ToMaterialCategoryEntity(m *generate.MaterialCategory) *entity.MaterialCategory {
	if m == nil {
		return nil
	}
	return &entity.MaterialCategory{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
