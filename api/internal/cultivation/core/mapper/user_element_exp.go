package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToUserElementExpResponse converts UserElementExp entity to response DTO.
func ToUserElementExpResponse(e *entity.UserElementExp) *dto.UserElementExpResponse {
	if e == nil {
		return nil
	}
	return &dto.UserElementExpResponse{
		ID:        e.ID,
		UserID:    e.UserID,
		ElementID: e.ElementID,
		Exp:       e.Exp,
	}
}

// ToUserElementExpEntityFromCreate converts CreateUserElementExpRequest to entity.
func ToUserElementExpEntityFromCreate(req *dto.CreateUserElementExpRequest) *entity.UserElementExp {
	return &entity.UserElementExp{
		UserID:    req.UserID,
		ElementID: req.ElementID,
		Exp:       req.Exp,
	}
}
