package entity

import "time"

// Permission represents a role-resource permission mapping.
type Permission struct {
	ID          int       `json:"id"`
	RoleID      int       `json:"role_id"`
	ResourceID  int       `json:"resource_id"`
	Description *string   `json:"description"`
	Scopes      int       `json:"scopes"` // Bitmask of allowed operations
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
