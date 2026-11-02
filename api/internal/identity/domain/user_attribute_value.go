package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// UserAttributeValue represents the value of a user attribute.
type UserAttributeValue struct {
	ID                   uuid.UUID            `json:"id"`
	UserID               uuid.UUID            `json:"user_id"`
	AttributeID          uuid.UUID            `json:"attribute_id"`
	DefinitionRevisionID uuid.UUID            `json:"definition_revision_id"`
	SourceRevisionID     uuid.UUID            `json:"source_revision_id"`
	CanonicalValue       json.RawMessage      `json:"canonical_value,omitempty"`
	SourceValueVersion   int64                `json:"source_value_version"`
	Version              int64                `json:"version"`
	Value                string               `json:"value"`
	Definition           *AttributeDefinition `json:"definition,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}
