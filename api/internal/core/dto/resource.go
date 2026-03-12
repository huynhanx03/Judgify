package dto

// CreateResourceRequest represents request to create a resource.
type CreateResourceRequest struct {
	Key         string  `json:"key" validate:"required,min=2,max=50"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// UpdateResourceRequest represents request to update a resource.
type UpdateResourceRequest struct {
	ID          int     `json:"-" uri:"id"`
	Key         *string `json:"key" validate:"omitempty,min=2,max=50"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// GetResourceRequest represents request to get a resource by ID.
type GetResourceRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteResourceRequest represents request to delete a resource.
type DeleteResourceRequest struct {
	ID int `uri:"id" validate:"required"`
}

// ResourceResponse represents resource data in API response.
type ResourceResponse struct {
	ID          int     `json:"id"`
	Key         string  `json:"key"`
	Description *string `json:"description"`
}
