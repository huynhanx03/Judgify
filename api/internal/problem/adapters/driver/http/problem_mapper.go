package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func toProblemID(request *problemIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse problem id: %w", err)
	}
	return id, nil
}

func toCreateProblemCommand(request *CreateProblemRequest) (contract.CreateProblemCommand, error) {
	difficultyID, err := uuid.Parse(request.DifficultyID)
	if err != nil {
		return contract.CreateProblemCommand{}, fmt.Errorf("parse problem difficulty id: %w", err)
	}
	tagIDs, err := parseProblemTagIDs(request.TagIDs)
	if err != nil {
		return contract.CreateProblemCommand{}, err
	}
	return contract.CreateProblemCommand{
		Slug: request.Slug, Title: request.Title, Description: request.Description,
		DifficultyID: difficultyID, TimeLimitMs: request.TimeLimitMs,
		MemoryLimitKb: request.MemoryLimitKb, TagIDs: tagIDs,
	}, nil
}

func parseProblemTagIDs(rawIDs []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(rawIDs))
	for index, rawID := range rawIDs {
		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("parse problem tag id at index %d: %w", index, err)
		}
		ids[index] = id
	}
	return ids, nil
}

func toProblemResponse(result *contract.ProblemResult) *ProblemResponse {
	if result == nil {
		return nil
	}
	response := &ProblemResponse{
		ID: result.ID.String(), Slug: result.Slug, Lifecycle: result.Lifecycle, Version: result.Version,
		Title: result.Title, Description: result.Description, DifficultyID: result.DifficultyID.String(),
		Difficulty: toDifficultyResponse(result.Difficulty), TimeLimitMs: result.TimeLimitMs,
		MemoryLimitKb: result.MemoryLimitKb, AuthorID: result.AuthorID.String(), IsPublished: result.IsPublished,
		SubmissionCount: result.SubmissionCount, AcceptedCount: result.AcceptedCount,
		AcceptanceRate: result.AcceptanceRate, IsSolved: result.IsSolved, CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt,
	}
	if result.Tags != nil {
		response.Tags = make([]*TagResponse, len(result.Tags))
		for index, tag := range result.Tags {
			response.Tags[index] = toTagResponse(tag)
		}
	}
	return response
}
