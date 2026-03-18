package dto

// CreateElementRequest represents request to create an element.
type CreateElementRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Code        string `json:"code" validate:"required,min=1,max=20"`
	Description string `json:"description" validate:"omitempty,max=255"`
	Color       string `json:"color" validate:"omitempty,max=20"`
	Icon        string `json:"icon" validate:"omitempty,max=100"`
	Order       int    `json:"order" validate:"min=0"`
}

// UpdateElementRequest represents request to update an element.
type UpdateElementRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Code        *string `json:"code" validate:"omitempty,min=1,max=20"`
	Description *string `json:"description" validate:"omitempty,max=255"`
	Color       *string `json:"color" validate:"omitempty,max=20"`
	Icon        *string `json:"icon" validate:"omitempty,max=100"`
	Order       *int    `json:"order" validate:"omitempty,min=0"`
}

// GetElementRequest represents request to get an element by ID.
type GetElementRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteElementRequest represents request to delete an element.
type DeleteElementRequest struct {
	ID int `uri:"id" validate:"required"`
}

// ElementResponse represents element data in API response.
type ElementResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order"`
}
