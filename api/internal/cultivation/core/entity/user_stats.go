package entity

import "time"

// UserStats represents user's cultivation state.
type UserStats struct {
	ID               int       `json:"id"`
	UserID           int       `json:"user_id"`
	TotalExp         int64     `json:"total_exp"`
	Rating           int       `json:"rating"`
	TotalSubmissions int       `json:"total_submissions"`
	AcceptedCount    int       `json:"accepted_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UserDifficultyStats tracks solved count per difficulty.
type UserDifficultyStats struct {
	UserID       int    `json:"user_id"`
	DifficultyID int    `json:"difficulty_id"`
	SolvedCount  int    `json:"solved_count"`
	// Eager-loaded
	DifficultyName  string `json:"difficulty_name"`
	DifficultyLevel int    `json:"difficulty_level"`
}

// UserTagStats tracks solved count per tag.
type UserTagStats struct {
	UserID      int    `json:"user_id"`
	TagID       int    `json:"tag_id"`
	SolvedCount int    `json:"solved_count"`
	// Eager-loaded
	TagName  string        `json:"tag_name"`
	Elements []ElementBrief `json:"elements"`
}

// ElementBrief is a minimal element for tag stats display.
type ElementBrief struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// UserStatsWithUser holds user stats with eager-loaded username.
type UserStatsWithUser struct {
	Stats    *UserStats
	Username string
}
