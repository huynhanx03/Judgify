package contract

import "github.com/google/uuid"

// UserTraitResponse represents a user trait result.
type UserTraitResponse struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	TraitID uuid.UUID
}
