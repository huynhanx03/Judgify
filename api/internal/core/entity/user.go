package entity

import "time"

// User represents a user in the Identity service.
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	RoleID    int       `json:"role_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
