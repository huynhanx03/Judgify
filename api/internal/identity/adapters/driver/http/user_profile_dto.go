package http

import (
	"encoding/json"
	"time"
)

type updateProfileAttributesRequest struct {
	Mutations []profileAttributeMutationRequest `json:"mutations" validate:"required,min=1,max=256,dive"`
}
type profileAttributeMutationRequest struct {
	DefinitionID                 string          `json:"definition_id" validate:"required"`
	ExpectedDefinitionRevisionID string          `json:"expected_definition_revision_id" validate:"required"`
	ExpectedValueVersion         int64           `json:"expected_value_version" validate:"gte=0"`
	Value                        json.RawMessage `json:"value,omitempty"`
	Unset                        bool            `json:"unset,omitempty"`
}
type profileAttrsResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Gender    int    `json:"gender"`
	Birthday  string `json:"birthday,omitempty"`
	JoinedAt  string `json:"joined_at,omitempty"`
}
type profileAttributeValueResponse struct {
	DefinitionID         string          `json:"definition_id"`
	DefinitionRevisionID string          `json:"definition_revision_id"`
	Key                  string          `json:"key"`
	Kind                 string          `json:"kind"`
	RequiredOnOnboarding bool            `json:"required_on_onboarding"`
	DataType             string          `json:"data_type"`
	Label                string          `json:"label"`
	Description          string          `json:"description"`
	Visibility           string          `json:"visibility"`
	UserEditable         bool            `json:"user_editable"`
	DisplayOrder         int             `json:"display_order"`
	Validation           json.RawMessage `json:"validation"`
	HasValue             bool            `json:"has_value"`
	Value                json.RawMessage `json:"value,omitempty"`
	ValueVersion         int64           `json:"value_version"`
}
type profileAttributesResponse struct {
	Profile    profileAttrsResponse            `json:"profile"`
	Attributes []profileAttributeValueResponse `json:"attributes"`
}
type userResponse struct {
	ID                    string     `json:"id"`
	Username              string     `json:"username"`
	Status                string     `json:"status"`
	Version               int64      `json:"version"`
	SuspendedAt           *time.Time `json:"suspended_at,omitempty"`
	Roles                 []string   `json:"roles"`
	AuthorizationRevision int64      `json:"authorization_revision"`
	CreatedAt             string     `json:"created_at"`
	UpdatedAt             string     `json:"updated_at"`
}
