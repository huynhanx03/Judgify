package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToTagEntity converts Ent Tag model to domain entity.
func ToTagEntity(m *generate.Tag) *entity.Tag {
	if m == nil {
		return nil
	}
	return &entity.Tag{
		ID:        m.ID,
		Name:      m.Name,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
