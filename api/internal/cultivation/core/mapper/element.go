package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToElementResponse converts Element entity to response DTO.
func ToElementResponse(e *entity.Element) *dto.ElementResponse {
	if e == nil {
		return nil
	}
	return &dto.ElementResponse{
		ID:          e.ID,
		Name:        e.Name,
		Code:        e.Code,
		Description: e.Description,
		Color:       e.Color,
		Icon:        e.Icon,
		Order:       e.Order,
	}
}

// ToElementEntityFromCreate converts CreateElementRequest to entity.
func ToElementEntityFromCreate(req *dto.CreateElementRequest) *entity.Element {
	return &entity.Element{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		Order:       req.Order,
	}
}
