package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToElementEntity converts Ent Element model to domain entity.
func ToElementEntity(m *generate.Element) *entity.Element {
	if m == nil {
		return nil
	}
	return &entity.Element{
		ID:          m.ID,
		Name:        m.Name,
		Code:        m.Code,
		Description: m.Description,
		Color:       m.Color,
		Icon:        m.Icon,
		Order:       m.Order,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
