package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus is a closed persisted authentication-session state.
type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusRevoked SessionStatus = "revoked"
	SessionStatusExpired SessionStatus = "expired"
)

// UserSecurityStatus is the closed account state relevant to authentication.
type UserSecurityStatus string

const (
	UserSecurityStatusInvited   UserSecurityStatus = "invited"
	UserSecurityStatusActive    UserSecurityStatus = "active"
	UserSecurityStatusSuspended UserSecurityStatus = "suspended"
	UserSecurityStatusDeleted   UserSecurityStatus = "deleted"
)

// Session is the complete server-owned lifecycle state for one client.
// IPHash and UserAgent are security-history metadata and must never be used as
// authentication authority.
type Session struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	FamilyID              uuid.UUID
	Status                SessionStatus
	UserSecurityVersion   uint64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	LastSeenAt            time.Time
	LastReauthenticatedAt time.Time
	IdleExpiresAt         time.Time
	AbsoluteExpiresAt     time.Time
	RevokedAt             *time.Time
	RevokeReason          *string `json:"-"`
	IPHash                []byte  `json:"-"`
	UserAgent             *string
	CreatedCID            string `json:"-"`
}

// SessionSecurityState is the minimal canonical session-plus-user projection
// needed to authenticate an already cryptographically verified access token.
// It intentionally excludes credentials, roles, permissions, and PII.
type SessionSecurityState struct {
	SessionID              uuid.UUID
	UserID                 uuid.UUID
	SessionStatus          SessionStatus
	SessionSecurityVersion uint64
	UserStatus             UserSecurityStatus
	UserSecurityVersion    uint64
	UserDeleted            bool
	Username               string
	LastSeenAt             time.Time
	LastReauthenticatedAt  time.Time
	IdleExpiresAt          time.Time
	AbsoluteExpiresAt      time.Time
}
