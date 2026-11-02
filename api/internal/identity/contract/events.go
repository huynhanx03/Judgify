package contract

import "time"

const (
	UserLifecyclePayloadVersionV1     = 1
	ProfileVisibilityPayloadVersionV1 = 1
	UserInvitationPayloadVersionV1    = 1
	AggregateUser                     = "identity.user"
	AggregateUserProfilePrivacy       = "identity_user_profile_privacy"
	AggregateUserInvitation           = "identity.user_invitation"
	EventUserSuspendedV1              = "identity.user.suspended.v1"
	EventUserReactivatedV1            = "identity.user.reactivated.v1"
	EventUserDeletedV1                = "identity.user.deleted.v1"
	EventProfileVisibilityChangedV1   = "identity.profile_visibility_changed.v1"
	EventUserInvitationCreatedV1      = "identity.user.invitation_created.v1"
	EventUserInvitationResentV1       = "identity.user.invitation_resent.v1"
	EventUserInvitationAcceptedV1     = "identity.user.invitation_accepted.v1"
)

// UserLifecycleStateV1 is the stable event projection for lifecycle changes.
type UserLifecycleStateV1 struct {
	UserID      string     `json:"user_id"`
	Username    string     `json:"username"`
	Status      string     `json:"status"`
	Version     int64      `json:"version"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
}

// UserLifecycleChangedV1 is the durable payload emitted by lifecycle commands.
type UserLifecycleChangedV1 struct {
	Version          int                  `json:"version"`
	Action           string               `json:"action"`
	CommandID        string               `json:"command_id"`
	RequestHash      string               `json:"request_hash"`
	AuditID          string               `json:"audit_id"`
	ActorID          string               `json:"actor_id"`
	ActorSessionID   string               `json:"actor_session_id"`
	ExpectedVersion  int64                `json:"expected_version"`
	ResultingVersion int64                `json:"resulting_version"`
	Before           UserLifecycleStateV1 `json:"before"`
	After            UserLifecycleStateV1 `json:"after"`
}

// UserLifecycleEventType maps a command action to its immutable event name.
func UserLifecycleEventType(action string) (string, bool) {
	switch action {
	case "suspend":
		return EventUserSuspendedV1, true
	case "reactivate":
		return EventUserReactivatedV1, true
	case "delete":
		return EventUserDeletedV1, true
	default:
		return "", false
	}
}

// ProfileVisibilityChangedV1 is the durable payload for privacy mutations.
type ProfileVisibilityChangedV1 struct {
	Version          int                   `json:"version"`
	ActorID          string                `json:"actor_id"`
	ActorSessionID   string                `json:"actor_session_id"`
	CommandID        string                `json:"command_id"`
	RequestHash      string                `json:"request_hash"`
	ExpectedVersion  int64                 `json:"expected_version"`
	ResultingVersion int64                 `json:"resulting_version"`
	AuditID          string                `json:"audit_id"`
	State            ProfilePrivacyStateV1 `json:"state"`
}

// ProfilePrivacyStateV1 belongs to the serialized event schema. Application
// responses intentionally remain transport-agnostic in public_profile.go.
type ProfilePrivacyStateV1 struct {
	Visibility string `json:"visibility"`
	Version    int64  `json:"version"`
	UpdatedAt  string `json:"updated_at"`
}

type UserInvitationCreatedV1 struct {
	Version             int                  `json:"version"`
	CommandID           string               `json:"command_id"`
	RequestHash         string               `json:"request_hash"`
	AuditID             string               `json:"audit_id"`
	ActorID             string               `json:"actor_id"`
	ActorSessionID      string               `json:"actor_session_id"`
	User                UserLifecycleStateV1 `json:"user"`
	Roles               []string             `json:"roles"`
	ProfileValueCount   int                  `json:"profile_value_count"`
	InvitationExpiresAt time.Time            `json:"invitation_expires_at"`
}

type UserInvitationResentV1 struct {
	Version                    int                  `json:"version"`
	CommandID                  string               `json:"command_id"`
	RequestHash                string               `json:"request_hash"`
	AuditID                    string               `json:"audit_id"`
	ActorID                    string               `json:"actor_id"`
	ActorSessionID             string               `json:"actor_session_id"`
	User                       UserLifecycleStateV1 `json:"user"`
	ExpectedInvitationVersion  uint64               `json:"expected_invitation_version"`
	ResultingInvitationVersion uint64               `json:"resulting_invitation_version"`
	InvitationExpiresAt        time.Time            `json:"invitation_expires_at"`
}

type UserInvitationAcceptedV1 struct {
	Version    int                  `json:"version"`
	AuditID    string               `json:"audit_id"`
	UserID     string               `json:"user_id"`
	Before     UserLifecycleStateV1 `json:"before"`
	After      UserLifecycleStateV1 `json:"after"`
	AcceptedAt time.Time            `json:"accepted_at"`
}
