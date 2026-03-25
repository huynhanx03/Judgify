package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToLevelResponse converts Level entity to response DTO.
func ToLevelResponse(e *entity.Level) *dto.LevelResponse {
	if e == nil {
		return nil
	}
	return &dto.LevelResponse{
		ID:          e.ID,
		Name:        e.Name,
		MinExp:      e.MinExp,
		Description: e.Description,
	}
}

// ToLevelEntityFromCreate converts CreateLevelRequest to entity.
func ToLevelEntityFromCreate(req *dto.CreateLevelRequest) *entity.Level {
	return &entity.Level{
		Name:        req.Name,
		MinExp:      req.MinExp,
		Description: req.Description,
	}
}
