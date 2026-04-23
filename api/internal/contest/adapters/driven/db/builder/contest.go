package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contest"
)

// BuildCreateContest builds the create mutation for Contest.
func BuildCreateContest(ctx context.Context, e *entity.Contest) *generate.ContestCreate {
	return global.EntClient.DB(ctx).Contest.Create().
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetStartTime(e.StartTime).
		SetEndTime(e.EndTime).
		SetStatus(contest.Status(e.Status)).
		SetAuthorID(e.AuthorID).
		SetMaxParticipants(e.MaxParticipants)
}

// BuildUpdateContest builds the update mutation for Contest.
func BuildUpdateContest(ctx context.Context, e *entity.Contest) *generate.ContestUpdateOne {
	return global.EntClient.DB(ctx).Contest.UpdateOneID(e.ID).
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetStartTime(e.StartTime).
		SetEndTime(e.EndTime).
		SetStatus(contest.Status(e.Status)).
		SetMaxParticipants(e.MaxParticipants)
}
