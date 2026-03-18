package dto

// CreateRankRequest represents request to create a rank.
type CreateRankRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Order       int    `json:"order" validate:"min=0"`
	MinRating   int    `json:"min_rating" validate:"min=0"`
	Description string `json:"description" validate:"omitempty,max=500"`
}

// UpdateRankRequest represents request to update a rank.
type UpdateRankRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	Order       *int    `json:"order" validate:"omitempty,min=0"`
	MinRating   *int    `json:"min_rating" validate:"omitempty,min=0"`
	Description *string `json:"description" validate:"omitempty,max=500"`
}

// GetRankRequest represents request to get a rank by ID.
type GetRankRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteRankRequest represents request to delete a rank.
type DeleteRankRequest struct {
	ID int `uri:"id" validate:"required"`
}

// RankResponse represents rank data in API response.
type RankResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Order       int    `json:"order"`
	MinRating   int    `json:"min_rating"`
	Description string `json:"description,omitempty"`
}
