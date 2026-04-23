package dto

// MaterialCategoryResponse represents material category data in API response.
type MaterialCategoryResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArticleCount int    `json:"article_count"`
}

// CreateMaterialCategoryRequest represents request to create a material category.
type CreateMaterialCategoryRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

// UpdateMaterialCategoryRequest represents request to update a material category.
type UpdateMaterialCategoryRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

// GetMaterialCategoryRequest represents request to get a material category by ID.
type GetMaterialCategoryRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteMaterialCategoryRequest represents request to delete a material category.
type DeleteMaterialCategoryRequest struct {
	ID int `uri:"id" validate:"required"`
}

// FindAllMaterialCategoriesRequest represents request to list all categories.
type FindAllMaterialCategoriesRequest struct{}
