package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserTrait represents a root bone or talent assigned to a user.
type UserTrait struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TraitID   uuid.UUID `json:"trait_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
