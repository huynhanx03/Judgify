package mapper

import (
	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
)

// ToMaterialCategoryResponse converts entity to API response.
func ToMaterialCategoryResponse(e *entity.MaterialCategory) *dto.MaterialCategoryResponse {
	if e == nil {
		return nil
	}
	return &dto.MaterialCategoryResponse{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		ArticleCount: e.ArticleCount,
	}
}
