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
	return &dto.TraitResponse{
		ID:          e.ID,
		Type:        e.Type,
		Name:        e.Name,
		RarityID:    e.RarityID,
		Description: e.Description,
		Metadata:    e.Metadata,
	}
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
