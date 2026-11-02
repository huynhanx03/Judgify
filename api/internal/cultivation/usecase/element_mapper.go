package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToElementResponse converts Element entity to response DTO.
func ToElementResponse(e *entity.Element) *contract.ElementResponse {
	if e == nil {
		return nil
	}
	return &contract.ElementResponse{
		ID:          e.ID,
		Name:        e.Name,
		Code:        e.Code,
		Description: e.Description,
		Version:     e.Version,
	}
}

// ToElementEntityFromCreate converts CreateElementRequest to entity.
func ToElementEntityFromCreate(req *contract.CreateElementCommand) *entity.Element {
	return &entity.Element{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
	}
}
