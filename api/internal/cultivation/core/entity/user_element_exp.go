package entity

import "time"

// UserElementExp tracks per-element EXP for a user.
type UserElementExp struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	ElementID int       `json:"element_id"`
	Exp       int64     `json:"exp"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
