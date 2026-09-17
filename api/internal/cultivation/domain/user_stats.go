package domain

import "github.com/google/uuid"

// InitialRating mirrors Contest's canonical expected-rank-v1 new-user value.
// Cultivation stores the projection; it does not independently recalculate it.
const InitialRating = 1200

// UserStats represents user's cultivation state.
type UserStats struct {
	UserID   uuid.UUID `json:"user_id"`
	TotalExp int64     `json:"total_exp"`
	Rating   int       `json:"rating"`
}
