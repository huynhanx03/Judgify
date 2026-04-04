package dto

// CreateUserStatsRequest represents request to init user stats.
type CreateUserStatsRequest struct {
	UserID   int   `json:"user_id" validate:"required"`
	TotalExp int64 `json:"total_exp" validate:"min=0"`
	Rating   int   `json:"rating" validate:"min=0"`
}

// UpdateUserStatsRequest represents request to update user stats.
type UpdateUserStatsRequest struct {
	ID       int    `json:"-" uri:"id"`
	TotalExp *int64 `json:"total_exp" validate:"omitempty,min=0"`
	Rating   *int   `json:"rating" validate:"omitempty,min=0"`
}

// GetUserStatsRequest represents request to get user stats by ID.
type GetUserStatsRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteUserStatsRequest represents request to delete user stats.
type DeleteUserStatsRequest struct {
	ID int `uri:"id" validate:"required"`
}

// UserStatsResponse represents user stats in API response.
type UserStatsResponse struct {
	ID       int   `json:"id"`
	UserID   int   `json:"user_id"`
	TotalExp int64 `json:"total_exp"`
	Rating   int   `json:"rating"`
}
