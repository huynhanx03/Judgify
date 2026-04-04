package entity

import "time"

// Difficulty represents a problem difficulty level with its exp reward.
type Difficulty struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Level       int       `json:"level"`
	ExpReward   int64     `json:"exp_reward"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
