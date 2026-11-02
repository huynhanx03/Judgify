package db

import (
	"github.com/huynhanx03/judgify/internal/ent/generate"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// BuildCreateProblem builds the create mutation for Problem entity.
func BuildCreateProblem(client *generate.Client, e *entity.Problem) *generate.ProblemCreate {
	return client.Problem.Create().
		SetSlug(e.Slug).
		SetTitle(e.Title).
		SetDescription(e.Description).
		SetDifficultyID(e.DifficultyID).
		SetTimeLimitMs(e.TimeLimitMs).
		SetMemoryLimitKB(e.MemoryLimitKb).
		SetAuthorID(e.AuthorID).
		SetIsPublished(e.IsPublished)
}
