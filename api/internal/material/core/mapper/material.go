package mapper

import (
	problemDto "github.com/huynhanx03/judgify/internal/problem/core/dto"
	problemMapper "github.com/huynhanx03/judgify/internal/problem/core/mapper"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// ToMaterialResponse converts entity to API response.
func ToMaterialResponse(e *entity.Material) *dto.MaterialResponse {
	if e == nil {
		return nil
	}
	resp := &dto.MaterialResponse{
		ID:                e.ID,
		Title:             e.Title,
		Description:       e.Description,
		Content:           e.Content,
		AuthorID:          e.AuthorID,
		Status:            e.Status,
		Visibility:        e.Visibility,
		ViewCount:         e.ViewCount,
		EstimatedReadTime: e.EstimatedReadTime,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}

	if e.Difficulty != nil {
		resp.Difficulty = &problemDto.DifficultyResponse{
			ID:          e.Difficulty.ID,
			Name:        e.Difficulty.Name,
			Level:       e.Difficulty.Level,
			ExpReward:   e.Difficulty.ExpReward,
			Description: e.Difficulty.Description,
		}
	}

	if e.Category != nil {
		resp.Category = ToMaterialCategoryResponse(e.Category)
	}

	if len(e.Tags) > 0 {
		resp.Tags = make([]*problemDto.TagResponse, len(e.Tags))
		for i, t := range e.Tags {
			resp.Tags[i] = problemMapper.ToTagResponse(&t)
		}
	}

	return resp
}
