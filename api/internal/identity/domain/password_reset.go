package domain

import (
	"time"

	"github.com/google/uuid"
)

// PasswordResetIntent is the minimum locked state required to consume one
// opaque reset credential. Raw credentials and recovery addresses never enter
// this entity.
type PasswordResetIntent struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	ContactID           uuid.UUID
	ContactVersion      uint64
	UserSecurityVersion uint64
	ExpiresAt           time.Time
}
