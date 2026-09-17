package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const MaximumPublicProfileAttributes = 256

// ProfileVisibility is the closed, fail-closed account discovery policy.
// Its zero value is invalid so an uninitialized projection can never become
// public accidentally.
type ProfileVisibility string

const (
	ProfileVisibilityPrivate ProfileVisibility = "private"
	ProfileVisibilityPublic  ProfileVisibility = "public"
)

func (visibility ProfileVisibility) Valid() bool {
	return visibility == ProfileVisibilityPrivate || visibility == ProfileVisibilityPublic
}

type ProfilePrivacyState struct {
	UserID     uuid.UUID
	Visibility ProfileVisibility
	Version    int64
	UpdatedAt  time.Time
}

func (state ProfilePrivacyState) Valid() bool {
	return state.UserID != uuid.Nil && state.Visibility.Valid() &&
		state.Version >= 1 && !state.UpdatedAt.IsZero()
}

// PublicProfile is an internal read model. UserID is retained only for
// consistency checks and is deliberately absent from the transport DTO.
type PublicProfile struct {
	UserID     uuid.UUID
	Username   string
	JoinedAt   time.Time
	Attributes []PublicProfileAttribute
}

// PublicProfileAttribute contains only stable display fields. Definition and
// revision identities, validation policy and value versions never cross the
// anonymous boundary.
type PublicProfileAttribute struct {
	Key         string
	DataType    AttributeValueType
	Label       string
	Description string
	Value       json.RawMessage
}
