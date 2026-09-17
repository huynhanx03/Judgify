package contract

import (
	"time"

	"github.com/google/uuid"
)

// SessionResponse is the bounded, non-secret session view returned to users
// and administrators. Refresh lineage, IP hashes, and revoke internals stay
// server-side.
type SessionResponse struct {
	ID                    uuid.UUID
	Current               bool
	Status                string
	CreatedAt             time.Time
	LastSeenAt            time.Time
	LastReauthenticatedAt time.Time
	IdleExpiresAt         time.Time
	AbsoluteExpiresAt     time.Time
	RevokedAt             *time.Time
	UserAgent             string
}

type SessionListRequest struct {
	Cursor string
	Limit  int
}

type SessionListResponse struct {
	Items      []SessionResponse
	NextCursor string
}

type SessionMutationResponse struct {
	Success bool
}
