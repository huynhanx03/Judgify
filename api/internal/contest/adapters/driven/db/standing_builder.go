package db

import (
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ent/generate"
)

// BuildCreateStanding builds the create mutation for ContestStanding.
func BuildCreateStanding(client *generate.Client, e *entity.ContestStanding) *generate.ContestStandingCreate {
	b := client.ContestStanding.Create().
		SetContestID(e.ContestID).
		SetUserID(e.UserID).
		SetSolvedCount(e.SolvedCount).
		SetPenalty(e.Penalty).
		SetNillableLastAcceptedElapsedMs(e.LastAcceptedElapsedMS).
		SetVersion(standingVersion(e.Version)).
		SetFrozenSolvedCount(e.FrozenSolvedCount).
		SetFrozenPenalty(e.FrozenPenalty).
		SetFrozenPendingAttempts(e.FrozenPendingAttemptCount).
		SetNillableFrozenLastAcceptedElapsedMs(e.FrozenLastAcceptedMS).
		SetFrozenVersion(standingVersion(e.FrozenVersion))
	if e.ProblemResults != nil {
		b.SetProblemResults(e.ProblemResults)
	}
	if e.FrozenProblemResults != nil {
		b.SetFrozenProblemResults(e.FrozenProblemResults)
	}
	if e.FrozenPendingByProblem != nil {
		b.SetFrozenPendingProblemAttempts(e.FrozenPendingByProblem)
	}
	return b
}

func standingVersion(value int64) int64 {
	if value < 1 {
		return 1
	}
	return value
}

// BuildUpdateStanding builds the update mutation for ContestStanding.
func BuildUpdateStanding(client *generate.Client, e *entity.ContestStanding) *generate.ContestStandingUpdateOne {
	b := client.ContestStanding.UpdateOneID(e.ID).
		SetSolvedCount(e.SolvedCount).
		SetPenalty(e.Penalty).
		SetNillableLastAcceptedElapsedMs(e.LastAcceptedElapsedMS).
		SetVersion(e.Version).
		SetFrozenSolvedCount(e.FrozenSolvedCount).
		SetFrozenPenalty(e.FrozenPenalty).
		SetFrozenPendingAttempts(e.FrozenPendingAttemptCount).
		SetNillableFrozenLastAcceptedElapsedMs(e.FrozenLastAcceptedMS).
		SetFrozenVersion(e.FrozenVersion)
	if e.LastAcceptedElapsedMS == nil {
		b.ClearLastAcceptedElapsedMs()
	}
	if e.FrozenLastAcceptedMS == nil {
		b.ClearFrozenLastAcceptedElapsedMs()
	}
	if e.ProblemResults != nil {
		b.SetProblemResults(e.ProblemResults)
	}
	if e.FrozenProblemResults != nil {
		b.SetFrozenProblemResults(e.FrozenProblemResults)
	}
	if e.FrozenPendingByProblem != nil {
		b.SetFrozenPendingProblemAttempts(e.FrozenPendingByProblem)
	}
	return b
}
