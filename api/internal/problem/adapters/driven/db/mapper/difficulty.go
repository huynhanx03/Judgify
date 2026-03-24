package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToDifficultyEntity converts Ent Difficulty model to domain entity.
func ToDifficultyEntity(m *generate.Difficulty) *entity.Difficulty {
	if m == nil {
		return nil
	}
	return &entity.Difficulty{
		ID:          m.ID,
		Name:        m.Name,
		Level:       m.Level,
		ExpReward:   m.ExpReward,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
