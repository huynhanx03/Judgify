package entity

import "time"

// Role represents a role with permissions.
type Role struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Level     int       `json:"level"`
	ParentID  int       `json:"parent_id"`
	Lft       int       `json:"lft"`
	Rgt       int       `json:"rgt"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
