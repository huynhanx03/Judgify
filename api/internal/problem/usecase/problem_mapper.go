package usecase

import (
	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
)

// ToProblemResult converts a Problem entity to its application projection.
// All nested data (difficulty, tags with elements) comes from the entity — no extra queries.
func ToProblemResult(e *entity.Problem) *contract.ProblemResult {
	if e == nil {
		return nil
	}
	resp := &contract.ProblemResult{
		ID:              e.ID,
		Slug:            e.Slug,
		Lifecycle:       string(e.Lifecycle),
		Version:         e.Version,
		Title:           e.Title,
		Description:     e.Description,
		DifficultyID:    e.DifficultyID,
		TimeLimitMs:     e.TimeLimitMs,
		MemoryLimitKb:   e.MemoryLimitKb,
		AuthorID:        e.AuthorID,
		IsPublished:     e.IsPublished,
		SubmissionCount: e.SubmissionCount,
		AcceptedCount:   e.AcceptedCount,
		IsSolved:        e.IsSolved,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
	if e.SubmissionCount > 0 {
		resp.AcceptanceRate = float64(e.AcceptedCount) * 100.0 / float64(e.SubmissionCount)
	}

	// Map difficulty from entity
	if e.Difficulty != nil {
		resp.Difficulty = &contract.DifficultyResult{
			ID: e.Difficulty.ID, Name: e.Difficulty.Name,
			Level: e.Difficulty.Level, ExpReward: e.Difficulty.ExpReward,
			Description: e.Difficulty.Description,
			Version:     e.Difficulty.Version,
		}
	}

	// Map tags with nested elements from entity
	if e.Tags != nil {
		resp.Tags = make([]*contract.TagResult, len(e.Tags))
		for i, t := range e.Tags {
			resp.Tags[i] = ToTagResult(&t)
		}
	}

	return resp
}

// ToProblemEntityFromCreate converts a CreateProblemCommand to a Problem entity.
func ToProblemEntityFromCreate(
	authorID uuid.UUID,
	req *contract.CreateProblemCommand,
) (*entity.Problem, error) {
	slug, err := entity.CanonicalProblemSlug(req.Slug)
	if err != nil {
		return nil, err
	}
	e := &entity.Problem{
		Slug:         slug,
		Title:        req.Title,
		Description:  req.Description,
		DifficultyID: req.DifficultyID,
		AuthorID:     authorID,
	}
	if req.TimeLimitMs != nil {
		e.TimeLimitMs = *req.TimeLimitMs
	} else {
		e.TimeLimitMs = problemcontract.DefaultCPUTimeLimitMilliseconds
	}
	if req.MemoryLimitKb != nil {
		e.MemoryLimitKb = *req.MemoryLimitKb
	} else {
		e.MemoryLimitKb = problemcontract.DefaultMemoryLimitKilobytes
	}
	return e, nil
}
