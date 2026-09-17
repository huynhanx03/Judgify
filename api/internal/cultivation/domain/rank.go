package domain

import (
	"time"

	"github.com/google/uuid"
)

// Rank represents a rating-based title.
type Rank struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MinRating   int       `json:"min_rating"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
