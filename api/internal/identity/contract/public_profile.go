package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type PublicProfileAttributeResponse struct {
	Key         string
	DataType    string
	Label       string
	Description string
	Value       json.RawMessage
}

// PublicProfileResponse is intentionally narrower than the authenticated
// profile. It carries no account/definition/revision identifiers, validation
// policy, value versions, roles, security state or internal provenance.
type PublicProfileResponse struct {
	Username   string
	JoinedAt   string
	Attributes []PublicProfileAttributeResponse
}

type ProfilePrivacyResponse struct {
	Visibility string
	Version    int64
	UpdatedAt  string
}

type ProfilePrivacyMutationReceipt struct {
	CommandID   uuid.UUID
	EventID     uuid.UUID
	State       ProfilePrivacyResponse
	CommittedAt time.Time
}
