package http

import (
	"encoding/json"
	"time"
)

type profileSchemaFieldResponse struct {
	DefinitionID         string          `json:"definition_id"`
	DefinitionRevisionID string          `json:"definition_revision_id"`
	Key                  string          `json:"key"`
	DataType             string          `json:"data_type"`
	Label                string          `json:"label"`
	Description          string          `json:"description"`
	Visibility           string          `json:"visibility"`
	UserEditable         bool            `json:"user_editable"`
	DisplayOrder         int             `json:"display_order"`
	Validation           json.RawMessage `json:"validation"`
}

type profileSchemaResponse struct {
	Revision   int64                        `json:"revision"`
	Checksum   string                       `json:"checksum"`
	Fields     []profileSchemaFieldResponse `json:"fields"`
	ServerTime time.Time                    `json:"server_time"`
}
