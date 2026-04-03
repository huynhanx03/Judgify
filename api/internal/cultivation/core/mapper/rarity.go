package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToRarityResponse converts Rarity entity to response DTO.
func ToRarityResponse(r *entity.Rarity) *dto.RarityResponse {
	if r == nil {
		return nil
	}
	return &dto.RarityResponse{
		ID:          r.ID,
		Name:        r.Name,
		Code:        r.Code,
		Weight:      r.Weight,
		Description: r.Description,
	}
}

// ToRarityEntityFromCreate converts CreateRarityRequest to entity.
func ToRarityEntityFromCreate(req *dto.CreateRarityRequest) *entity.Rarity {
	return &entity.Rarity{
		Name:        req.Name,
		Code:        req.Code,
		Weight:      req.Weight,
		Description: req.Description,
	}
}
