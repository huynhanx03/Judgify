package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
)

// ToStandingResponse converts entity to API response with rank.
func ToStandingResponse(rank int, e *entity.ContestStanding, username string) *contract.StandingResponse {
	if e == nil {
		return nil
	}
	return &contract.StandingResponse{
		Rank:             rank,
		UserID:           e.UserID,
		Username:         username,
		SolvedCount:      e.SolvedCount,
		Penalty:          e.Penalty,
		ProblemResults:   e.ProblemResults,
		PendingAttempts:  e.VisiblePendingAttempts,
		PendingByProblem: e.VisiblePendingByProblem,
	}
}
