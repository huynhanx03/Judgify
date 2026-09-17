package contract

import "github.com/google/uuid"

// MaterialCategoryResult is the application projection of a material category.
type MaterialCategoryResult struct {
	ID           uuid.UUID
	Name         string
	Description  string
	ArticleCount int
}

// CreateMaterialCategoryCommand is the application input for creating a category.
type CreateMaterialCategoryCommand struct {
	Name        string
	Description string
}

// UpdateMaterialCategoryCommand is the application input for updating a category.
type UpdateMaterialCategoryCommand struct {
	Name        *string
	Description *string
}
