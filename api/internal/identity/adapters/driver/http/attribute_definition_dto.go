package http

import (
	"encoding/json"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"time"
)

type createAttributeDefinitionRequest struct {
	CommandID            string          `json:"command_id" validate:"required"`
	Reason               string          `json:"reason" validate:"required,min=3,max=500"`
	Key                  string          `json:"key" validate:"required,min=2,max=50"`
	DataType             string          `json:"data_type" validate:"required,oneof=string number boolean date"`
	Label                string          `json:"label" validate:"required,min=2,max=128"`
	Description          string          `json:"description" validate:"max=1024"`
	Visibility           string          `json:"visibility" validate:"required,oneof=public private admin"`
	UserEditable         bool            `json:"user_editable"`
	DisplayOrder         int             `json:"display_order" validate:"gte=0,lte=10000"`
	RequiredOnOnboarding bool            `json:"required_on_onboarding"`
	Validation           json.RawMessage `json:"validation" validate:"required"`
}
type updateAttributeDefinitionRequest struct {
	ID                   string          `uri:"id" validate:"required"`
	CommandID            string          `json:"command_id" validate:"required"`
	ExpectedVersion      int64           `json:"expected_version" validate:"required,gt=0"`
	Reason               string          `json:"reason" validate:"required,min=3,max=500"`
	DataType             *string         `json:"data_type" validate:"omitempty,oneof=string number boolean date"`
	Label                *string         `json:"label" validate:"omitempty,min=2,max=128"`
	Description          *string         `json:"description" validate:"omitempty,max=1024"`
	Visibility           *string         `json:"visibility" validate:"omitempty,oneof=public private admin"`
	UserEditable         *bool           `json:"user_editable"`
	DisplayOrder         *int            `json:"display_order" validate:"omitempty,gte=0,lte=10000"`
	RequiredOnOnboarding *bool           `json:"required_on_onboarding"`
	Validation           json.RawMessage `json:"validation,omitempty"`
}
type getAttributeDefinitionRequest struct {
	ID string `uri:"id" validate:"required"`
}
type listAttributeDefinitionRevisionsRequest struct {
	ID     string `uri:"id" validate:"required"`
	Cursor string `form:"cursor" validate:"omitempty,max=32"`
	Limit  int    `form:"limit" validate:"omitempty,min=1,max=100"`
}
type attributeLifecycleCommandRequest struct {
	ID                  string `uri:"id" validate:"required"`
	CommandID           string `json:"command_id" validate:"required"`
	ExpectedVersion     int64  `json:"expected_version" validate:"required,gt=0"`
	CandidateRevisionID string `json:"candidate_revision_id,omitempty"`
	Reason              string `json:"reason" validate:"required,min=3,max=500"`
	ReviewedPreviewHash string `json:"review_hash,omitempty" validate:"omitempty,len=64,hexadecimal"`
	ConfirmationToken   string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}
type attributeDefinitionRevisionResponse struct {
	ID           string          `json:"id"`
	DefinitionID string          `json:"definition_id"`
	Revision     int64           `json:"revision"`
	DataType     string          `json:"data_type"`
	Label        string          `json:"label"`
	Description  string          `json:"description"`
	Visibility   string          `json:"visibility"`
	UserEditable bool            `json:"user_editable"`
	DisplayOrder int             `json:"display_order"`
	Validation   json.RawMessage `json:"validation"`
	CreatedAt    time.Time       `json:"created_at"`
}
type attributeDefinitionResponse struct {
	ID                   string                               `json:"id"`
	Key                  string                               `json:"key"`
	Kind                 string                               `json:"kind"`
	RequiredOnOnboarding bool                                 `json:"required_on_onboarding"`
	Status               string                               `json:"status"`
	Version              int64                                `json:"version"`
	DraftRevisionID      *string                              `json:"draft_revision_id"`
	ActiveRevisionID     *string                              `json:"active_revision_id"`
	DraftRevision        *attributeDefinitionRevisionResponse `json:"draft_revision"`
	ActiveRevision       *attributeDefinitionRevisionResponse `json:"active_revision"`
}
type attributeDefinitionRevisionPageResponse struct {
	Records    []*attributeDefinitionRevisionResponse `json:"records"`
	NextCursor string                                 `json:"next_cursor,omitempty"`
}
type attributeLifecyclePreviewResponse struct {
	Preview struct {
		Action            string                               `json:"action"`
		DefinitionID      string                               `json:"definition_id"`
		ExpectedVersion   int64                                `json:"expected_version"`
		ResultingVersion  int64                                `json:"resulting_version"`
		CurrentRevision   *attributeDefinitionRevisionResponse `json:"current_revision,omitempty"`
		CandidateRevision *attributeDefinitionRevisionResponse `json:"candidate_revision,omitempty"`
		SourceValueCount  int64                                `json:"source_value_count"`
		TargetValueCount  int64                                `json:"target_value_count"`
		RequiresMigration bool                                 `json:"requires_migration"`
	} `json:"preview"`
	ReviewHash        string    `json:"review_hash"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}
type attributeLifecycleReceiptResponse struct {
	Status           string                               `json:"status"`
	Action           string                               `json:"action"`
	CommandID        string                               `json:"command_id"`
	EventID          *string                              `json:"event_id,omitempty"`
	DefinitionID     string                               `json:"definition_id"`
	Version          int64                                `json:"version"`
	Definition       *attributeDefinitionResponse         `json:"definition,omitempty"`
	ActiveRevision   *attributeDefinitionRevisionResponse `json:"active_revision,omitempty"`
	Operation        *operation.Projection                `json:"operation,omitempty"`
	RecordedAt       time.Time                            `json:"recorded_at"`
	IdempotentReplay bool                                 `json:"idempotent_replay"`
}
