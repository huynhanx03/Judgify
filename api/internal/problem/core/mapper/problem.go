package mapper

import (
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToProblemResponse converts Problem entity to ProblemResponse DTO.
// All nested data (difficulty, tags with elements) comes from the entity — no extra queries.
func ToProblemResponse(e *entity.Problem) *dto.ProblemResponse {
	if e == nil {
		return nil
	}
	resp := &dto.ProblemResponse{
		ID:            e.ID,
		Title:         e.Title,
		Description:   e.Description,
		DifficultyID:  e.DifficultyID,
		TimeLimitMs:   e.TimeLimitMs,
		MemoryLimitKb: e.MemoryLimitKb,
		AuthorID:      e.AuthorID,
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
		resp.Difficulty = &dto.DifficultyResponse{
			ID: e.Difficulty.ID, Name: e.Difficulty.Name,
			Level: e.Difficulty.Level, ExpReward: e.Difficulty.ExpReward,
			Description: e.Difficulty.Description,
		}
	}

	// Map tags with nested elements from entity
	if e.Tags != nil {
		resp.Tags = make([]*dto.TagResponse, len(e.Tags))
		for i, t := range e.Tags {
			resp.Tags[i] = ToTagResponse(&t)
		}
	}

	return resp
}

// ToProblemEntityFromCreate converts CreateProblemRequest to Problem entity.
func ToProblemEntityFromCreate(authorID int, req *dto.CreateProblemRequest) *entity.Problem {
	e := &entity.Problem{
		Title:        req.Title,
		Description:  req.Description,
		DifficultyID: req.DifficultyID,
		AuthorID:     authorID,
	}
	if req.TimeLimitMs != nil {
		e.TimeLimitMs = *req.TimeLimitMs
	} else {
		e.TimeLimitMs = 1000 // default 1s
	}
	if req.MemoryLimitKb != nil {
		e.MemoryLimitKb = *req.MemoryLimitKb
	} else {
		e.MemoryLimitKb = 262144 // default 256MB
	}
	return e
}
