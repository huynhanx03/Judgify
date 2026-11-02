package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToUserTraitResponse converts UserTrait entity to response DTO.
func ToUserTraitResponse(e *entity.UserTrait) *contract.UserTraitResponse {
	if e == nil {
		return nil
	}
	return &contract.UserTraitResponse{
		ID:      e.ID,
		UserID:  e.UserID,
		TraitID: e.TraitID,
	}
}
