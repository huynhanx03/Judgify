package http

// CreateMaterialCategoryRequest is the HTTP payload for creating a category.
type CreateMaterialCategoryRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

// UpdateMaterialCategoryRequest is the HTTP payload for updating a category.
type UpdateMaterialCategoryRequest struct {
	ID          string  `json:"-" uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

type materialCategoryIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

// MaterialCategoryResponse is the HTTP representation of a category.
type MaterialCategoryResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArticleCount int    `json:"article_count"`
}
