package entity

import "time"

// FederatedIdentity represents external OAuth/OIDC identity.
type FederatedIdentity struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Provider   string    `json:"provider"`    // google, facebook, github, etc.
	ExternalID string    `json:"external_id"` // User ID from external provider
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
