package entity

import "time"

// UserAttributeValue represents the value of a user attribute.
type UserAttributeValue struct {
	ID          int                  `json:"id"`
	UserID      int                  `json:"user_id"`
	AttributeID int                  `json:"attribute_id"`
	Value       string               `json:"value"`
	Definition  *AttributeDefinition `json:"definition,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}
