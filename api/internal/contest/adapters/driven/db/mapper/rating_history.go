package mapper

import (
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToRatingHistoryEntity converts Ent RatingHistory model to domain entity.
func ToRatingHistoryEntity(m *generate.RatingHistory) *entity.RatingHistory {
	if m == nil {
		return nil
	}
	return &entity.RatingHistory{
		ID:           m.ID,
		UserID:       m.UserID,
		ContestID:    m.ContestID,
		OldRating:    m.OldRating,
		NewRating:    m.NewRating,
		RankPosition: m.RankPosition,
	}
}
