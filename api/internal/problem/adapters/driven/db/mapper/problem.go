package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToProblemEntity converts Ent Problem model to domain entity.
func ToProblemEntity(m *generate.Problem) *entity.Problem {
	if m == nil {
		return nil
	}
	return &entity.Problem{
		ID:            m.ID,
		Title:         m.Title,
		Description:   m.Description,
		Difficulty:    string(m.Difficulty),
		TimeLimitMs:   m.TimeLimitMs,
		MemoryLimitKb: m.MemoryLimitKB,
		AuthorID:      m.AuthorID,
		IsPublished:   m.IsPublished,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}
