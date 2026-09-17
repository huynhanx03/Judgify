package usecase

import (
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// ToMaterialCategoryResult converts an entity to its application projection.
func ToMaterialCategoryResult(e *entity.MaterialCategory) *contract.MaterialCategoryResult {
	if e == nil {
		return nil
	}
	return &contract.MaterialCategoryResult{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		ArticleCount: e.ArticleCount,
	}
}
