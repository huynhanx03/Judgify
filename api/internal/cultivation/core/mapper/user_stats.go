package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToUserStatsResponse converts UserStats entity to response DTO.
func ToUserStatsResponse(e *entity.UserStats) *dto.UserStatsResponse {
	if e == nil {
		return nil
	}
	return &dto.UserStatsResponse{
		ID:             e.ID,
		UserID:         e.UserID,
		TotalExp:       e.TotalExp,
		CurrentLevelID: e.CurrentLevelID,
		Rating:         e.Rating,
	}
}

// ToUserStatsEntityFromCreate converts CreateUserStatsRequest to entity.
func ToUserStatsEntityFromCreate(req *dto.CreateUserStatsRequest) *entity.UserStats {
	return &entity.UserStats{
		UserID:         req.UserID,
		CurrentLevelID: req.CurrentLevelID,
		TotalExp:       req.TotalExp,
		Rating:         req.Rating,
	}
}
