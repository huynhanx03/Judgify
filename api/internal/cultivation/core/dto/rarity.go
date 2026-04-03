package dto

// CreateRarityRequest represents request to create a rarity.
type CreateRarityRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=50"`
	Code        string `json:"code" validate:"required,min=1,max=20"`
	Weight      int    `json:"weight" validate:"omitempty,min=1"`
	Description string `json:"description" validate:"omitempty,max=255"`
}

// UpdateRarityRequest represents request to update a rarity.
type UpdateRarityRequest struct {
	ID          int     `json:"-" uri:"id"`
	Name        *string `json:"name" validate:"omitempty,min=1,max=50"`
	Code        *string `json:"code" validate:"omitempty,min=1,max=20"`
	Weight      *int    `json:"weight" validate:"omitempty,min=1"`
	Description *string `json:"description" validate:"omitempty,max=255"`
}

// GetRarityRequest represents request to get a rarity by ID.
type GetRarityRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteRarityRequest represents request to delete a rarity.
type DeleteRarityRequest struct {
	ID int `uri:"id" validate:"required"`
}

// FindAllRaritiesRequest is an empty request for GET /rarities.
type FindAllRaritiesRequest struct{}

// RarityResponse represents rarity data in API response.
type RarityResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Weight      int    `json:"weight"`
	Description string `json:"description,omitempty"`
}
