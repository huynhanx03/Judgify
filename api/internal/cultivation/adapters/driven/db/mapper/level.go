package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToLevelEntity converts Ent Level model to domain entity.
func ToLevelEntity(m *generate.Level) *entity.Level {
	if m == nil {
		return nil
	}
	return &entity.Level{
		ID:          m.ID,
		Name:        m.Name,
		MinExp:      m.MinExp,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
