package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/operation"
)

const (
	AttributeLifecycleCommitted          = "committed"
	AttributeLifecycleMigrationScheduled = "migration_scheduled"
)

// CreateAttributeDefinitionRequest represents request to create an attribute definition.
type CreateAttributeDefinitionRequest struct {
	CommandID            uuid.UUID
	Reason               string
	Key                  string
	DataType             string
	Label                string
	Description          string
	Visibility           string
	UserEditable         bool
	DisplayOrder         int
	RequiredOnOnboarding bool
	Validation           json.RawMessage
}

// UpdateAttributeDefinitionRequest updates mutable presentation fields only.
// Key is intentionally absent because it is the definition's stable identity.
type UpdateAttributeDefinitionRequest struct {
	ID                   uuid.UUID
	CommandID            uuid.UUID
	ExpectedVersion      int64
	Reason               string
	DataType             *string
	Label                *string
	Description          *string
	Visibility           *string
	UserEditable         *bool
	DisplayOrder         *int
	RequiredOnOnboarding *bool
	Validation           json.RawMessage
}

type AttributeDefinitionRevisionResponse struct {
	ID           uuid.UUID
	DefinitionID uuid.UUID
	Revision     int64
	DataType     string
	Label        string
	Description  string
	Visibility   string
	UserEditable bool
	DisplayOrder int
	Validation   json.RawMessage
	CreatedAt    time.Time
}

type AttributeLifecycleCommandRequest struct {
	ID                  uuid.UUID
	CommandID           uuid.UUID
	ExpectedVersion     int64
	CandidateRevisionID uuid.UUID
	Reason              string
	ReviewedPreviewHash string
	ConfirmationToken   string
}

type AttributeLifecyclePreview struct {
	Action            string
	DefinitionID      uuid.UUID
	ExpectedVersion   int64
	ResultingVersion  int64
	CurrentRevision   *AttributeDefinitionRevisionResponse
	CandidateRevision *AttributeDefinitionRevisionResponse
	SourceValueCount  int64
	TargetValueCount  int64
	RequiresMigration bool
}

type AttributeLifecyclePreviewResponse struct {
	Preview           AttributeLifecyclePreview
	ReviewHash        string
	ConfirmationToken string
	ExpiresAt         time.Time
}

type ListAttributeDefinitionRevisionsRequest struct {
	ID     uuid.UUID
	Cursor string
	Limit  int
}

type AttributeDefinitionRevisionPage struct {
	Records    []*AttributeDefinitionRevisionResponse
	NextCursor string
}

type AttributeLifecycleReceipt struct {
	Status           string
	Action           string
	CommandID        uuid.UUID
	EventID          *uuid.UUID
	DefinitionID     uuid.UUID
	Version          int64
	Definition       *AttributeDefinitionResponse
	ActiveRevision   *AttributeDefinitionRevisionResponse
	Operation        *operation.Projection
	RecordedAt       time.Time
	IdempotentReplay bool
}

// GetAttributeDefinitionRequest represents request to get an attribute definition by ID.
type GetAttributeDefinitionRequest struct {
	ID uuid.UUID
}

// AttributeDefinitionResponse represents attribute definition data in API response.
type AttributeDefinitionResponse struct {
	ID                   uuid.UUID
	Key                  string
	Kind                 string
	RequiredOnOnboarding bool
	Status               string
	Version              int64
	DraftRevisionID      *uuid.UUID
	ActiveRevisionID     *uuid.UUID
	DraftRevision        *AttributeDefinitionRevisionResponse
	ActiveRevision       *AttributeDefinitionRevisionResponse
}
