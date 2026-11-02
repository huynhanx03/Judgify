package domain

import (
	"time"

	"github.com/google/uuid"
)

// FederatedIdentity represents external OAuth/OIDC identity.
type FederatedIdentity struct {
	ID                  uuid.UUID  `json:"id"`
	UserID              uuid.UUID  `json:"user_id"`
	Provider            string     `json:"provider"`
	ProviderSubject     string     `json:"provider_subject"`
	DisplayName         *string    `json:"display_name,omitempty"`
	AvatarURL           *string    `json:"avatar_url,omitempty"`
	LastAuthenticatedAt *time.Time `json:"last_authenticated_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
