package http

// CreateDifficultyRequest is the HTTP payload for creating a difficulty.
type CreateDifficultyRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Level       int    `json:"level" validate:"required,min=1"`
	ExpReward   int64  `json:"exp_reward" validate:"omitempty,min=0"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// UpdateDifficultyRequest is the HTTP payload for updating a difficulty.
type UpdateDifficultyRequest struct {
	ID          string  `json:"-" uri:"id" validate:"required,uuid"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Level       *int    `json:"level" validate:"omitempty,min=1"`
	ExpReward   *int64  `json:"exp_reward" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

type difficultyIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}

// DifficultyResponse is the HTTP representation of a difficulty.
type DifficultyResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	ExpReward   int64  `json:"exp_reward"`
	Description string `json:"description,omitempty"`
	Version     int64  `json:"version"`
}
