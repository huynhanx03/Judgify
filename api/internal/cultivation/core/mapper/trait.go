package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToTraitResponse converts Trait entity to response DTO.
func ToTraitResponse(e *entity.Trait) *dto.TraitResponse {
	if e == nil {
		return nil
	}
	resp := &dto.TraitResponse{
		ID:          e.ID,
		Type:        e.Type,
		Name:        e.Name,
		Description: e.Description,
		Metadata:    e.Metadata,
	}
	if e.Rarity != nil {
		resp.Rarity = &dto.TraitRarityInfo{
			ID:     e.Rarity.ID,
			Name:   e.Rarity.Name,
			Code:   e.Rarity.Code,
			Weight: e.Rarity.Weight,
		}
	}
	return resp
}

// ToTraitEntityFromCreate converts CreateTraitRequest to entity.
func ToTraitEntityFromCreate(req *dto.CreateTraitRequest) *entity.Trait {
	return &entity.Trait{
		Type:        req.Type,
		Name:        req.Name,
		RarityID:    req.RarityID,
		Description: req.Description,
		Metadata:    req.Metadata,
	}
}
