package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToLevelResponse converts Level entity to response DTO.
func ToLevelResponse(e *entity.Level) *contract.LevelResponse {
	if e == nil {
		return nil
	}
	return &contract.LevelResponse{
		ID:          e.ID,
		Name:        e.Name,
		MinExp:      e.MinExp,
		Description: e.Description,
	}
}

// ToLevelEntityFromCreate converts CreateLevelCommand to entity.
func ToLevelEntityFromCreate(command *contract.CreateLevelCommand) *entity.Level {
	return &entity.Level{
		Name:        command.Name,
		MinExp:      command.MinExp,
		Description: command.Description,
	}
}
