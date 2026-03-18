package dto

// CreateUserTraitRequest represents request to assign a trait to a user.
type CreateUserTraitRequest struct {
	UserID  int `json:"user_id" validate:"required"`
	TraitID int `json:"trait_id" validate:"required"`
}

// GetUserTraitRequest represents request to get a user trait by ID.
type GetUserTraitRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteUserTraitRequest represents request to remove a user trait.
type DeleteUserTraitRequest struct {
	ID int `uri:"id" validate:"required"`
}

// UserTraitResponse represents user trait data in API response.
type UserTraitResponse struct {
	ID      int `json:"id"`
	UserID  int `json:"user_id"`
	TraitID int `json:"trait_id"`
}
