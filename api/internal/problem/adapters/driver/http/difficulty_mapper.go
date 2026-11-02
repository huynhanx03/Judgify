package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func toCreateDifficultyCommand(request *CreateDifficultyRequest) contract.CreateDifficultyCommand {
	return contract.CreateDifficultyCommand{
		Name: request.Name, Level: request.Level, ExpReward: request.ExpReward, Description: request.Description,
	}
}

func toUpdateDifficultyCommand(request *UpdateDifficultyRequest) (uuid.UUID, contract.UpdateDifficultyCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, contract.UpdateDifficultyCommand{}, fmt.Errorf("parse difficulty id: %w", err)
	}
	return id, contract.UpdateDifficultyCommand{
		Name: request.Name, Level: request.Level, ExpReward: request.ExpReward, Description: request.Description,
	}, nil
}

func toDifficultyID(request *difficultyIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse difficulty id: %w", err)
	}
	return id, nil
}

func toDifficultyResponse(result *contract.DifficultyResult) *DifficultyResponse {
	if result == nil {
		return nil
	}
	return &DifficultyResponse{ID: result.ID.String(), Name: result.Name, Level: result.Level, ExpReward: result.ExpReward, Description: result.Description, Version: result.Version}
}
