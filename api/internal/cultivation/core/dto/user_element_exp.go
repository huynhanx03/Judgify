package dto

// CreateUserElementExpRequest represents request to create/init element EXP.
type CreateUserElementExpRequest struct {
	UserID    int   `json:"user_id" validate:"required"`
	ElementID int   `json:"element_id" validate:"required"`
	Exp       int64 `json:"exp" validate:"min=0"`
}

// UpdateUserElementExpRequest represents request to update element EXP.
type UpdateUserElementExpRequest struct {
	ID  int    `json:"-" uri:"id"`
	Exp *int64 `json:"exp" validate:"omitempty,min=0"`
}

// GetUserElementExpRequest represents request to get by ID.
type GetUserElementExpRequest struct {
	ID int `uri:"id" validate:"required"`
}

// DeleteUserElementExpRequest represents request to delete.
type DeleteUserElementExpRequest struct {
	ID int `uri:"id" validate:"required"`
}

// UserElementExpResponse represents user element EXP in API response.
type UserElementExpResponse struct {
	ID        int   `json:"id"`
	UserID    int   `json:"user_id"`
	ElementID int   `json:"element_id"`
	Exp       int64 `json:"exp"`
}
