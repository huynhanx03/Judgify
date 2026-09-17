package http

import (
	"encoding/json"
	"time"
)

type getPublicProfileRequest struct {
	Username string `uri:"username" validate:"required,min=3,max=50,alphanum"`
}
type updateProfilePrivacyRequest struct {
	Visibility      string `json:"visibility" validate:"required,oneof=private public"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
}
type publicProfileAttributeResponse struct {
	Key         string          `json:"key"`
	DataType    string          `json:"data_type"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Value       json.RawMessage `json:"value"`
}
type publicProfileResponse struct {
	Username   string                           `json:"username"`
	JoinedAt   string                           `json:"joined_at"`
	Attributes []publicProfileAttributeResponse `json:"attributes"`
}
type profilePrivacyResponse struct {
	Visibility string `json:"visibility"`
	Version    int64  `json:"version"`
	UpdatedAt  string `json:"updated_at"`
}
type profilePrivacyMutationReceipt struct {
	CommandID   string                 `json:"command_id"`
	EventID     string                 `json:"event_id"`
	State       profilePrivacyResponse `json:"state"`
	CommittedAt time.Time              `json:"committed_at"`
}
