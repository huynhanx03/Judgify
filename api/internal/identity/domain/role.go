package domain

import (
	"time"

	"github.com/google/uuid"
)

type RoleStatus string

const (
	RoleStatusActive   RoleStatus = "active"
	RoleStatusArchived RoleStatus = "archived"
)

func (status RoleStatus) Valid() bool {
	return status == RoleStatusActive || status == RoleStatusArchived
}

// Role represents a role with permissions.
type Role struct {
	ID          uuid.UUID  `json:"id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	IsSystem    bool       `json:"is_system"`
	Status      RoleStatus `json:"status"`
	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
