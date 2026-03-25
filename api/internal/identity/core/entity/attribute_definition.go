package entity

import "time"

// AttributeDefinition represents the definition of a user attribute.
type AttributeDefinition struct {
	ID          int       `json:"id"`
	Key         string    `json:"key"`
	DataType    string    `json:"data_type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
