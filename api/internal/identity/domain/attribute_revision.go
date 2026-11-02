package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AttributeValueType string

const (
	AttributeValueTypeString  AttributeValueType = "string"
	AttributeValueTypeNumber  AttributeValueType = "number"
	AttributeValueTypeBoolean AttributeValueType = "boolean"
	AttributeValueTypeDate    AttributeValueType = "date"
)

func (value AttributeValueType) Valid() bool {
	return value == AttributeValueTypeString || value == AttributeValueTypeNumber ||
		value == AttributeValueTypeBoolean || value == AttributeValueTypeDate
}

type AttributeVisibility string

const (
	AttributeVisibilityPublic  AttributeVisibility = "public"
	AttributeVisibilityPrivate AttributeVisibility = "private"
	AttributeVisibilityAdmin   AttributeVisibility = "admin"
)

func (value AttributeVisibility) Valid() bool {
	return value == AttributeVisibilityPublic || value == AttributeVisibilityPrivate ||
		value == AttributeVisibilityAdmin
}

// AttributeDefinitionRevision is immutable. Updating a definition always
// appends a new revision and moves only the draft/active pointer.
type AttributeDefinitionRevision struct {
	ID           uuid.UUID
	DefinitionID uuid.UUID
	Revision     int64
	DataType     AttributeValueType
	Label        string
	Description  string
	Visibility   AttributeVisibility
	UserEditable bool
	DisplayOrder int
	Validation   json.RawMessage
	CreatedBy    *uuid.UUID
	CreatedAt    time.Time
}
