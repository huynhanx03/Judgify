package dto

// RegisterRequest represents request to register for a contest.
type RegisterRequest struct {
	ContestID int `uri:"id" validate:"required"`
}

// UnregisterRequest represents request to unregister from a contest.
type UnregisterRequest struct {
	ContestID int `uri:"id" validate:"required"`
}
