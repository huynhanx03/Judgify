package contract

import (
	"time"

	"github.com/google/uuid"
)

// UserInvitationCommandRequest is shared by preview/apply. Actor/session and
// the canonical Idempotency-Key are bound by the HTTP adapter, never JSON.
type UserInvitationCommandRequest struct {
	ProfileSchemaEvidence
	CommandID         uuid.UUID
	Username          string
	RecoveryEmail     string
	Roles             []string
	Reason            string
	ConfirmationToken string
}

type UserInvitationPreview struct {
	Username          string
	MaskedDestination string
	Roles             []string
	ProfileValueCount int
}

type UserInvitationPreviewResponse struct {
	Preview           UserInvitationPreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

type UserInvitationReceipt struct {
	CommandID           uuid.UUID
	EventID             uuid.UUID
	User                UserLifecycleProjection
	Roles               []string
	ProfileValueCount   int
	InvitationExpiresAt time.Time
	DeliveryState       string
	CommittedAt         time.Time
	IdempotentReplay    bool
}

type UserInvitationResendCommandRequest struct {
	ID                        uuid.UUID
	CommandID                 uuid.UUID
	ExpectedUserVersion       int64
	ExpectedInvitationVersion uint64
	Reason                    string
	ConfirmationToken         string
}

type UserInvitationResendPreview struct {
	User                       UserLifecycleProjection
	MaskedDestination          string
	ExpectedInvitationVersion  uint64
	ResultingInvitationVersion uint64
	CurrentExpiresAt           time.Time
}

type UserInvitationResendPreviewResponse struct {
	Preview           UserInvitationResendPreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

type UserInvitationResendReceipt struct {
	CommandID           uuid.UUID
	EventID             uuid.UUID
	User                UserLifecycleProjection
	InvitationVersion   uint64
	InvitationExpiresAt time.Time
	DeliveryState       string
	CommittedAt         time.Time
	IdempotentReplay    bool
}

type ActivateUserInvitationRequest struct {
	Token       string
	NewPassword string
}

type ActivateUserInvitationResponse struct {
	Success bool
}
