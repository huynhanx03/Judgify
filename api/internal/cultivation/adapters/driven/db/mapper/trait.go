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
	t := &entity.Trait{
		ID:          m.ID,
		Type:        string(m.Type),
		Name:        m.Name,
		RarityID:    m.RarityID,
		Description: m.Description,
		Metadata:    m.Metadata,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	if r, err := m.Edges.RarityOrErr(); err == nil && r != nil {
		t.Rarity = &entity.TraitRarity{
			ID:     r.ID,
			Name:   r.Name,
			Code:   r.Code,
			Weight: r.Weight,
		}
	}
	return t
}
