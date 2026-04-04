package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToProblemEntity converts Ent Problem model to domain entity.
// Maps eager-loaded edges (tags with elements, difficulty) in a single pass.
func ToProblemEntity(m *generate.Problem) *entity.Problem {
	if m == nil {
		return nil
	}
	p := &entity.Problem{
		ID:            m.ID,
		Title:         m.Title,
		Description:   m.Description,
		DifficultyID:  m.DifficultyID,
		TimeLimitMs:   m.TimeLimitMs,
		MemoryLimitKb: m.MemoryLimitKB,
		AuthorID:      m.AuthorID,
		IsPublished:     m.IsPublished,
		SubmissionCount: m.SubmissionCount,
		AcceptedCount:   m.AcceptedCount,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}

	// Map difficulty from edge
	if d := m.Edges.Difficulty; d != nil {
		p.Difficulty = &entity.ProblemDifficulty{
			ID: d.ID, Name: d.Name, Level: d.Level,
			ExpReward: d.ExpReward, Description: d.Description,
		}
	}

	// Map tags with nested elements from edges
	if m.Edges.Tags != nil {
		p.Tags = make([]entity.Tag, len(m.Edges.Tags))
		for i, t := range m.Edges.Tags {
			tag := entity.Tag{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
			if t.Edges.Elements != nil {
				tag.Elements = make([]entity.TagElement, len(t.Edges.Elements))
				for j, e := range t.Edges.Elements {
					tag.Elements[j] = entity.TagElement{ID: e.ID, Name: e.Name, Code: e.Code}
				}
			}
			p.Tags[i] = tag
		}
	}

	return p
}
