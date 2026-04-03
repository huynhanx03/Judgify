package dto

// CreateLevelRequest represents request to create a cultivation level.
type CreateLevelRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	MinExp      int64  `json:"min_exp" validate:"min=0"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

// UpdateLevelRequest represents request to update a level.
type UpdateLevelRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	MinExp      *int64  `json:"min_exp" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

// GetLevelRequest represents request to get a level by ID.
type GetLevelRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteLevelRequest represents request to delete a level.
type DeleteLevelRequest struct {
	ID int `uri:"id" validate:"required"`
}

// FindAllLevelsRequest is an empty request for GET /levels.
type FindAllLevelsRequest struct{}

// LevelResponse represents level data in API response.
type LevelResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	MinExp      int64  `json:"min_exp"`
	Description string `json:"description,omitempty"`
}
