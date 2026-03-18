package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToUserTraitResponse converts UserTrait entity to response DTO.
func ToUserTraitResponse(e *entity.UserTrait) *dto.UserTraitResponse {
	if e == nil {
		return nil
	}
	return &dto.UserTraitResponse{
		ID:      e.ID,
		UserID:  e.UserID,
		TraitID: e.TraitID,
	}
}

// ToUserTraitEntityFromCreate converts CreateUserTraitRequest to entity.
func ToUserTraitEntityFromCreate(req *dto.CreateUserTraitRequest) *entity.UserTrait {
	return &entity.UserTrait{
		UserID:  req.UserID,
		TraitID: req.TraitID,
	}
}
