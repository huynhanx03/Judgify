package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToRankEntity converts Ent Rank model to domain entity.
func ToRankEntity(m *generate.Rank) *entity.Rank {
	if m == nil {
		return nil
	}
	return &entity.Rank{
		ID:          m.ID,
		Name:        m.Name,
		Order:       m.Order,
		MinRating:   m.MinRating,
		Description: m.Description,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
