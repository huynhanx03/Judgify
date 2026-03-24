package mapper

import (
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
)

// ToDifficultyResponse converts Difficulty entity to DifficultyResponse DTO.
func ToDifficultyResponse(e *entity.Difficulty) *dto.DifficultyResponse {
	if e == nil {
		return nil
	}
	return &dto.DifficultyResponse{
		ID:          e.ID,
		Name:        e.Name,
		Level:       e.Level,
		ExpReward:   e.ExpReward,
		Description: e.Description,
	}
}

// ToDifficultyEntityFromCreate converts CreateDifficultyRequest to Difficulty entity.
func ToDifficultyEntityFromCreate(req *dto.CreateDifficultyRequest) *entity.Difficulty {
	return &entity.Difficulty{
		Name:        req.Name,
		Level:       req.Level,
		ExpReward:   req.ExpReward,
		Description: req.Description,
	}
}
