package mapper

import (
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
)

// ToStandingResponse converts entity to API response with rank.
func ToStandingResponse(rank int, e *entity.ContestStanding, username string) *dto.StandingResponse {
	if e == nil {
		return nil
	}
	return &dto.StandingResponse{
		Rank:           rank,
		UserID:         e.UserID,
		Username:       username,
		SolvedCount:    e.SolvedCount,
		Penalty:        e.Penalty,
		ProblemResults: e.ProblemResults,
	}
}
