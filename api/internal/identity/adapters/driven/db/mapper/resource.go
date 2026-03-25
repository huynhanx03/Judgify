package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
)

// ToResourceEntity converts Ent Resource model to domain entity.
func ToResourceEntity(m *generate.Resource) *entity.Resource {
	if m == nil {
		return nil
	}
	return &entity.Resource{
		ID:          m.ID,
		Key:         m.Key,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// ToResourceModel converts domain entity to Ent Resource model.
func ToResourceModel(e *entity.Resource) *generate.Resource {
	if e == nil {
		return nil
	}
	return &generate.Resource{
		ID:          e.ID,
		Key:         e.Key,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
