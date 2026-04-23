package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	problemEntity "github.com/huynhanx03/judgify/internal/problem/core/entity"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// ToMaterialEntity converts Ent Material model to domain entity.
func ToMaterialEntity(m *generate.Material) *entity.Material {
	if m == nil {
		return nil
	}
	e := &entity.Material{
		ID:                m.ID,
		Title:             m.Title,
		Description:       m.Description,
		Content:           m.Content,
		DifficultyID:      m.DifficultyID,
		AuthorID:          m.AuthorID,
		CategoryID:        m.CategoryID,
		Status:            string(m.Status),
		Visibility:        string(m.Visibility),
		GroupID:           m.GroupID,
		ViewCount:         m.ViewCount,
		EstimatedReadTime: m.EstimatedReadTime,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}

	if d := m.Edges.Difficulty; d != nil {
		e.Difficulty = &problemEntity.ProblemDifficulty{
			ID:          d.ID,
			Name:        d.Name,
			Level:       d.Level,
			ExpReward:   d.ExpReward,
			Description: d.Description,
		}
	}

	if c := m.Edges.Category; c != nil {
		e.Category = ToMaterialCategoryEntity(c)
	}

	if m.Edges.Tags != nil {
		e.Tags = make([]problemEntity.Tag, len(m.Edges.Tags))
		for i, t := range m.Edges.Tags {
			tag := problemEntity.Tag{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}
			if t.Edges.Elements != nil {
				tag.Elements = make([]problemEntity.TagElement, len(t.Edges.Elements))
				for j, el := range t.Edges.Elements {
					tag.Elements[j] = problemEntity.TagElement{ID: el.ID, Name: el.Name, Code: el.Code}
				}
			}
			e.Tags[i] = tag
		}
	}

	return e
}
