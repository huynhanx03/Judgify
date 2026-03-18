package entity

import "time"

// Rank represents a rating-based title.
type Rank struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	MinRating   int       `json:"min_rating"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
