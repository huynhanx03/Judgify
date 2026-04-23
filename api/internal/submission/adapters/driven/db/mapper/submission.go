package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
)

// ToSubmissionEntity converts Ent Submission model to domain entity.
func ToSubmissionEntity(m *generate.Submission) *entity.Submission {
	if m == nil {
		return nil
	}
	return &entity.Submission{
		ID:           m.ID,
		ProblemID:    m.ProblemID,
		UserID:       m.UserID,
		Language:     string(m.Language),
		SourceCode:   m.SourceCode,
		Status:       string(m.Status),
		PassedCount:  m.PassedCount,
		TotalCount:   m.TotalCount,
		TimeMs:       m.TimeMs,
		MemoryKb:     m.MemoryKB,
		ContestID:    m.ContestID,
		ErrorMessage: m.ErrorMessage,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
