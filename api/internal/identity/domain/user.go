package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the Identity service.
type User struct {
	ID                         uuid.UUID          `json:"id"`
	Username                   string             `json:"username"`
	SecurityStatus             UserSecurityStatus `json:"status"`
	SecurityVersion            uint64             `json:"-"`
	SuspendedAt                *time.Time         `json:"suspended_at,omitempty"`
	SuspensionReason           string             `json:"-"`
	ProfileVisibility          ProfileVisibility  `json:"profile_visibility"`
	ProfileVisibilityVersion   int64              `json:"profile_visibility_version"`
	ProfileVisibilityUpdatedAt time.Time          `json:"profile_visibility_updated_at"`
	CreatedAt                  time.Time          `json:"created_at"`
	UpdatedAt                  time.Time          `json:"updated_at"`
}
