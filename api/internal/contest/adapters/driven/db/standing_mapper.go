package db

import (
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// ToStandingEntity converts Ent ContestStanding model to domain entity.
func ToStandingEntity(m *generate.ContestStanding) *entity.ContestStanding {
	if m == nil {
		return nil
	}
	e := &entity.ContestStanding{
		ID:                        m.ID,
		ContestID:                 m.ContestID,
		UserID:                    m.UserID,
		SolvedCount:               m.SolvedCount,
		Penalty:                   m.Penalty,
		LastAcceptedElapsedMS:     m.LastAcceptedElapsedMs,
		Version:                   m.Version,
		FrozenSolvedCount:         m.FrozenSolvedCount,
		FrozenPenalty:             m.FrozenPenalty,
		FrozenPendingAttemptCount: m.FrozenPendingAttempts,
		FrozenLastAcceptedMS:      m.FrozenLastAcceptedElapsedMs,
		FrozenVersion:             m.FrozenVersion,
	}
	if user, err := m.Edges.UserOrErr(); err == nil && user != nil {
		e.Username = user.Username
	}
	if m.ProblemResults != nil {
		e.ProblemResults = m.ProblemResults
	}
	if m.FrozenProblemResults != nil {
		e.FrozenProblemResults = m.FrozenProblemResults
	}
	if m.FrozenPendingProblemAttempts != nil {
		e.FrozenPendingByProblem = m.FrozenPendingProblemAttempts
	}
	return e
}
