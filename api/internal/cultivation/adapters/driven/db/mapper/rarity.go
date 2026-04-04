package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToRarityEntity converts Ent Rarity model to domain entity.
func ToRarityEntity(m *generate.Rarity) *entity.Rarity {
	if m == nil {
		return nil
	}
	return &entity.Rarity{
		ID:          m.ID,
		Name:        m.Name,
		Code:        m.Code,
		Weight:      m.Weight,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
