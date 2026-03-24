package dto

// CreateDifficultyRequest represents request to create a difficulty.
type CreateDifficultyRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Level       int    `json:"level" validate:"required,min=1"`
	ExpReward   int64  `json:"exp_reward" validate:"omitempty,min=0"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// UpdateDifficultyRequest represents request to update a difficulty.
type UpdateDifficultyRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Level       *int    `json:"level" validate:"omitempty,min=1"`
	ExpReward   *int64  `json:"exp_reward" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// GetDifficultyRequest represents request to get a difficulty by ID.
type GetDifficultyRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteDifficultyRequest represents request to delete a difficulty.
type DeleteDifficultyRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DifficultyResponse represents difficulty data in API response.
type DifficultyResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	ExpReward   int64  `json:"exp_reward"`
	Description string `json:"description,omitempty"`
}
