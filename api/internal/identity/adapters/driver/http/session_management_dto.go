package http

import "time"

type sessionListQuery struct {
	Cursor string
	Limit  int
}

type sessionResponse struct {
	ID                    string     `json:"id"`
	Current               bool       `json:"current"`
	Status                string     `json:"status"`
	CreatedAt             time.Time  `json:"created_at"`
	LastSeenAt            time.Time  `json:"last_seen_at"`
	LastReauthenticatedAt time.Time  `json:"last_reauthenticated_at"`
	IdleExpiresAt         time.Time  `json:"idle_expires_at"`
	AbsoluteExpiresAt     time.Time  `json:"absolute_expires_at"`
	RevokedAt             *time.Time `json:"revoked_at,omitempty"`
	UserAgent             string     `json:"user_agent,omitempty"`
}

type sessionListResponse struct {
	Items      []sessionResponse `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type sessionMutationResponse struct {
	Success bool `json:"success"`
}
