package builder

import (
	"context"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// BuildCreateProblem builds the create mutation for Problem entity.
func BuildCreateProblem(ctx context.Context, e *entity.Problem) *generate.ProblemCreate {
	return global.EntClient.DB(ctx).Problem.Create().
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetDifficultyID(e.DifficultyID).
		SetTimeLimitMs(e.TimeLimitMs).
		SetMemoryLimitKB(e.MemoryLimitKb).
		SetAuthorID(e.AuthorID).
		SetIsPublished(e.IsPublished)
}

// BuildUpdateProblem builds the update mutation for Problem entity.
func BuildUpdateProblem(ctx context.Context, e *entity.Problem) *generate.ProblemUpdateOne {
	return global.EntClient.DB(ctx).Problem.UpdateOneID(e.ID).
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetDifficultyID(e.DifficultyID).
		SetTimeLimitMs(e.TimeLimitMs).
		SetMemoryLimitKB(e.MemoryLimitKb).
		SetIsPublished(e.IsPublished)
}
