package mapper

import (
	"github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
)

// ToRankResponse converts Rank entity to response DTO.
func ToRankResponse(e *entity.Rank) *dto.RankResponse {
	if e == nil {
		return nil
	}
	return &dto.RankResponse{
		ID:          e.ID,
		Name:        e.Name,
		MinRating:   e.MinRating,
		Description: e.Description,
	}
}

// ToRankEntityFromCreate converts CreateRankRequest to entity.
func ToRankEntityFromCreate(req *dto.CreateRankRequest) *entity.Rank {
	return &entity.Rank{
		Name:        req.Name,
		MinRating:   req.MinRating,
		Description: req.Description,
	}
}
