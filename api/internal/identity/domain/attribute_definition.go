package domain

import (
	"time"

	"github.com/google/uuid"
)

// AttributeDefinition represents the definition of a user attribute.
type AttributeDefinition struct {
	ID                   uuid.UUID                    `json:"id"`
	Key                  string                       `json:"key"`
	Kind                 string                       `json:"kind"`
	RequiredOnOnboarding bool                         `json:"required_on_onboarding"`
	Status               string                       `json:"status"`
	Version              int64                        `json:"version"`
	DraftRevisionID      *uuid.UUID                   `json:"draft_revision_id,omitempty"`
	ActiveRevisionID     *uuid.UUID                   `json:"active_revision_id,omitempty"`
	DraftRevision        *AttributeDefinitionRevision `json:"draft_revision,omitempty"`
	ActiveRevision       *AttributeDefinitionRevision `json:"active_revision,omitempty"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
}
