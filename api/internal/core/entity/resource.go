package entity

import "time"

// Resource represents a protected resource for RBAC.
type Resource struct {
	ID          int       `json:"id"`
	Key         string    `json:"key"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
