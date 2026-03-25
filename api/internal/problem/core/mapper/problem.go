package mapper

import (
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToProblemResponse converts Problem entity to ProblemResponse DTO.
func ToProblemResponse(e *entity.Problem) *dto.ProblemResponse {
	if e == nil {
		return nil
	}
	return &dto.ProblemResponse{
		ID:            e.ID,
		Title:         e.Title,
		Description:   e.Description,
		DifficultyID:  e.DifficultyID,
		TimeLimitMs:   e.TimeLimitMs,
		MemoryLimitKb: e.MemoryLimitKb,
		AuthorID:      e.AuthorID,
		IsPublished:   e.IsPublished,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
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
