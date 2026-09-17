package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type userInvitationProfileValueRequest struct {
	DefinitionID         uuid.UUID       `json:"definition_id" validate:"required"`
	DefinitionRevisionID uuid.UUID       `json:"definition_revision_id" validate:"required"`
	Value                json.RawMessage `json:"value" validate:"required"`
}

type userInvitationCommandRequest struct {
	CommandID             uuid.UUID                           `json:"command_id" validate:"required"`
	Username              string                              `json:"username" validate:"required,min=3,max=50,alphanum"`
	RecoveryEmail         string                              `json:"recovery_email" validate:"required,email,max=320"`
	Roles                 []string                            `json:"roles" validate:"required,min=1,max=64,dive,min=2,max=50"`
	Reason                string                              `json:"reason" validate:"required,min=3,max=1024"`
	ConfirmationToken     string                              `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
	ProfileSchemaRevision int64                               `json:"profile_schema_revision" validate:"required,gt=0"`
	ProfileSchemaChecksum string                              `json:"profile_schema_checksum" validate:"required,len=64,hexadecimal"`
	ProfileValues         []userInvitationProfileValueRequest `json:"profile_values" validate:"required,min=1,max=128,dive"`
}

type userInvitationResendCommandRequest struct {
	ID                        string `uri:"id" validate:"required"`
	CommandID                 string `json:"command_id" validate:"required"`
	ExpectedUserVersion       int64  `json:"expected_user_version" validate:"required,gt=0"`
	ExpectedInvitationVersion uint64 `json:"expected_invitation_version" validate:"gte=0"`
	Reason                    string `json:"reason" validate:"required,min=3,max=1024"`
	ConfirmationToken         string `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type userInvitationPreviewResponse struct {
	Preview struct {
		Username          string   `json:"username"`
		MaskedDestination string   `json:"masked_destination"`
		Roles             []string `json:"roles"`
		ProfileValueCount int      `json:"profile_value_count"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type userInvitationReceiptResponse struct {
	CommandID           string                          `json:"command_id"`
	EventID             string                          `json:"event_id"`
	User                userLifecycleProjectionResponse `json:"user"`
	Roles               []string                        `json:"roles"`
	ProfileValueCount   int                             `json:"profile_value_count"`
	InvitationExpiresAt time.Time                       `json:"invitation_expires_at"`
	DeliveryState       string                          `json:"delivery_state"`
	CommittedAt         time.Time                       `json:"committed_at"`
	IdempotentReplay    bool                            `json:"idempotent_replay"`
}

type userInvitationResendPreviewResponse struct {
	Preview struct {
		User                       userLifecycleProjectionResponse `json:"user"`
		MaskedDestination          string                          `json:"masked_destination"`
		ExpectedInvitationVersion  uint64                          `json:"expected_invitation_version"`
		ResultingInvitationVersion uint64                          `json:"resulting_invitation_version"`
		CurrentExpiresAt           time.Time                       `json:"current_expires_at"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type userInvitationResendReceiptResponse struct {
	CommandID           string                          `json:"command_id"`
	EventID             string                          `json:"event_id"`
	User                userLifecycleProjectionResponse `json:"user"`
	InvitationVersion   uint64                          `json:"invitation_version"`
	InvitationExpiresAt time.Time                       `json:"invitation_expires_at"`
	DeliveryState       string                          `json:"delivery_state"`
	CommittedAt         time.Time                       `json:"committed_at"`
	IdempotentReplay    bool                            `json:"idempotent_replay"`
}
