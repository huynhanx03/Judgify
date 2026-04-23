package mapper

import (
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToStandingEntity converts Ent ContestStanding model to domain entity.
func ToStandingEntity(m *generate.ContestStanding) *entity.ContestStanding {
	if m == nil {
		return nil
	}
	e := &entity.ContestStanding{
		ID:          m.ID,
		ContestID:   m.ContestID,
		UserID:      m.UserID,
		SolvedCount: m.SolvedCount,
		Penalty:     m.Penalty,
	}
	if m.ProblemResults != nil {
		e.ProblemResults = m.ProblemResults
	}
	return e
}
