package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProfileValueInput binds one typed value to the exact immutable definition
// revision the browser rendered. Value must contain one JSON scalar.
type ProfileValueInput struct {
	DefinitionID         uuid.UUID
	DefinitionRevisionID uuid.UUID
	Value                json.RawMessage
}

// ProfileSchemaEvidence is echoed by account-creation commands. The service
// revalidates the exact required set while holding the schema fence.
type ProfileSchemaEvidence struct {
	ProfileSchemaRevision int64
	ProfileSchemaChecksum string
	ProfileValues         []ProfileValueInput
}

type ProfileSchemaField struct {
	DefinitionID         uuid.UUID
	DefinitionRevisionID uuid.UUID
	Key                  string
	DataType             string
	Label                string
	Description          string
	Visibility           string
	UserEditable         bool
	DisplayOrder         int
	Validation           json.RawMessage
}

type ProfileSchemaResponse struct {
	Revision   int64
	Checksum   string
	Fields     []ProfileSchemaField
	ServerTime time.Time
}
