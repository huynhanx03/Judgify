package domain

import (
	"time"

	"github.com/google/uuid"
)

// Rarity represents a cultivation rarity tier (e.g. mortal, earth, heaven, divine).
type Rarity struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Weight      int       `json:"weight"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
