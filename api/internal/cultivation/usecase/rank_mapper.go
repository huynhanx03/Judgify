package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToRankResponse converts Rank entity to response DTO.
func ToRankResponse(e *entity.Rank) *contract.RankResponse {
	if e == nil {
		return nil
	}
	return &contract.RankResponse{
		ID:          e.ID,
		Name:        e.Name,
		MinRating:   e.MinRating,
		Description: e.Description,
	}
}

// ToRankEntityFromCreate converts CreateRankCommand to entity.
func ToRankEntityFromCreate(command *contract.CreateRankCommand) *entity.Rank {
	return &entity.Rank{
		Name:        command.Name,
		MinRating:   command.MinRating,
		Description: command.Description,
	}
}
