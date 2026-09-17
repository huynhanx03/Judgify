package domain

import "github.com/google/uuid"

// UserElementExp tracks per-element EXP for a user.
type UserElementExp struct {
	UserID    uuid.UUID `json:"user_id"`
	ElementID uuid.UUID `json:"element_id"`
	Exp       int64     `json:"exp"`
}
