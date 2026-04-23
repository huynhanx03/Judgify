package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateStanding builds the create mutation for ContestStanding.
func BuildCreateStanding(ctx context.Context, e *entity.ContestStanding) *generate.ContestStandingCreate {
	b := global.EntClient.DB(ctx).ContestStanding.Create().
		SetContestID(e.ContestID).
		SetUserID(e.UserID).
		SetSolvedCount(e.SolvedCount).
		SetPenalty(e.Penalty)
	if e.ProblemResults != nil {
		b.SetProblemResults(e.ProblemResults)
	}
	return b
}

// BuildUpdateStanding builds the update mutation for ContestStanding.
func BuildUpdateStanding(ctx context.Context, e *entity.ContestStanding) *generate.ContestStandingUpdateOne {
	b := global.EntClient.DB(ctx).ContestStanding.UpdateOneID(e.ID).
		SetSolvedCount(e.SolvedCount).
		SetPenalty(e.Penalty)
	if e.ProblemResults != nil {
		b.SetProblemResults(e.ProblemResults)
	}
	return b
}
