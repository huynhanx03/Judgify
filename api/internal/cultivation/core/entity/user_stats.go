package entity

import "time"

// UserStats represents user's cultivation state.
type UserStats struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	TotalExp       int64     `json:"total_exp"`
	CurrentLevelID int       `json:"current_level_id"`
	Rating         int       `json:"rating"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
