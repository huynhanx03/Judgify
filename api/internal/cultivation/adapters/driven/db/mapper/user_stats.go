package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToUserStatsEntity converts Ent UserStats model to domain entity.
func ToUserStatsEntity(m *generate.UserStats) *entity.UserStats {
	if m == nil {
		return nil
	}
	return &entity.UserStats{
		ID:               m.ID,
		UserID:           m.UserID,
		TotalExp:         m.TotalExp,
		Rating:           m.Rating,
		TotalSubmissions: m.TotalSubmissions,
		AcceptedCount:    m.AcceptedCount,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
