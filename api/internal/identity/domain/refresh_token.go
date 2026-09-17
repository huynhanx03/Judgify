package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshTokenStatus is a closed single-use refresh credential state.
type RefreshTokenStatus string

const (
	RefreshTokenStatusIssued   RefreshTokenStatus = "issued"
	RefreshTokenStatusConsumed RefreshTokenStatus = "consumed"
	RefreshTokenStatusRevoked  RefreshTokenStatus = "revoked"
)

// RefreshToken contains only a keyed hash and rotation lineage. Raw refresh
// credentials are never represented by this persistence entity.
type RefreshToken struct {
	ID           uuid.UUID
	SessionID    uuid.UUID
	TokenHash    []byte `json:"-"`
	ParentID     *uuid.UUID
	Status       RefreshTokenStatus
	IssuedAt     time.Time
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	ReplacedByID *uuid.UUID
	RevokedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
