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
	var elements []entity.TagElement
	if m.Edges.Elements != nil {
		elements = make([]entity.TagElement, len(m.Edges.Elements))
		for i, e := range m.Edges.Elements {
			elements[i] = entity.TagElement{ID: e.ID, Name: e.Name, Code: e.Code}
		}
	}
	return &entity.Tag{
		ID:        m.ID,
		Name:      m.Name,
		Elements:  elements,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
