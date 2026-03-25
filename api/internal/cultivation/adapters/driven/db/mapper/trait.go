package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToTraitEntity converts Ent Trait model to domain entity.
func ToTraitEntity(m *generate.Trait) *entity.Trait {
	if m == nil {
		return nil
	}
	return &entity.Trait{
		ID:          m.ID,
		Type:        string(m.Type),
		Name:        m.Name,
		RarityID:    m.RarityID,
		Description: m.Description,
		Metadata:    m.Metadata,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
