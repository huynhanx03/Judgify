package mapper

import (
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToContestEntity converts Ent Contest model to domain entity.
func ToContestEntity(m *generate.Contest) *entity.Contest {
	if m == nil {
		return nil
	}
	return &entity.Contest{
		ID:              m.ID,
		Title:           m.Title,
		Description:     m.Description,
		StartTime:       m.StartTime,
		EndTime:         m.EndTime,
		Status:          string(m.Status),
		AuthorID:        m.AuthorID,
		MaxParticipants: m.MaxParticipants,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
