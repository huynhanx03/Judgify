package domain

import (
	"time"

	"github.com/google/uuid"
)

// Level represents a cultivation realm.
type Level struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MinExp      int64     `json:"min_exp"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
