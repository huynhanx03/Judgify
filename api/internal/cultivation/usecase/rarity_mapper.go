package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToRarityResponse converts Rarity entity to response DTO.
func ToRarityResponse(r *entity.Rarity) *contract.RarityResponse {
	if r == nil {
		return nil
	}
	return &contract.RarityResponse{
		ID:          r.ID,
		Name:        r.Name,
		Code:        r.Code,
		Weight:      r.Weight,
		Description: r.Description,
	}
}

// ToRarityEntityFromCreate converts CreateRarityCommand to entity.
func ToRarityEntityFromCreate(command *contract.CreateRarityCommand) *entity.Rarity {
	return &entity.Rarity{
		Name:        command.Name,
		Code:        command.Code,
		Weight:      command.Weight,
		Description: command.Description,
	}
}
