package entity

import "time"

// Credential represents user authentication credentials.
type Credential struct {
	ID             int                    `json:"id"`
	UserID         int                    `json:"user_id"`
	Type           string                 `json:"type"`            // password, totp, webauthn, etc.
	CredentialData map[string]interface{} `json:"credential_data"` // Encrypted credential data (JSON)
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}
