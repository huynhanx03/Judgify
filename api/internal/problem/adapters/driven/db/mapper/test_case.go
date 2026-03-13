package mapper

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToTestCaseEntity converts Ent TestCase model to domain entity.
func ToTestCaseEntity(m *generate.TestCase) *entity.TestCase {
	if m == nil {
		return nil
	}
	return &entity.TestCase{
		ID:             m.ID,
		ProblemID:      m.ProblemID,
		Input:          m.Input,
		ExpectedOutput: m.ExpectedOutput,
		IsSample:       m.IsSample,
		OrderIndex:     m.OrderIndex,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
