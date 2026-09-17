package http

import (
	"time"

	"github.com/google/uuid"
)

const recoveryContactVerificationStatusVerified = "verified"
const reauthenticationStatusVerified = "reauthenticated"

type verifyRecoveryContactRequest struct {
	Token string `json:"token" validate:"required,max=4096"`
}

type verifyRecoveryContactResponse struct {
	Status string `json:"status"`
}

type recoveryContactCurrentResponse struct {
	Status        string `json:"status"`
	MaskedAddress string `json:"masked_address"`
	Version       uint64 `json:"version"`
}

type recoveryContactPendingResponse struct {
	Purpose               string    `json:"purpose"`
	Status                string    `json:"status"`
	MaskedAddress         string    `json:"masked_address"`
	Version               uint64    `json:"version"`
	VerificationExpiresAt time.Time `json:"verification_expires_at"`
	ResendAvailableAt     time.Time `json:"resend_available_at"`
	DeliveryState         string    `json:"delivery_state"`
}

type recoveryContactResponse struct {
	Current              *recoveryContactCurrentResponse `json:"current"`
	Pending              *recoveryContactPendingResponse `json:"pending"`
	ReauthenticatedUntil *time.Time                      `json:"reauthenticated_until"`
	CanRemove            bool                            `json:"can_remove"`
}

type reauthenticateRecoveryContactRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=256"`
}

type reauthenticateRecoveryContactResponse struct {
	Status               string    `json:"status"`
	ReauthenticatedUntil time.Time `json:"reauthenticated_until"`
}

type replaceRecoveryContactRequest struct {
	RecoveryEmail          string    `json:"recovery_email" validate:"required,max=320"`
	ExpectedCurrentVersion uint64    `json:"expected_current_version" validate:"required,min=1"`
	IdempotencyKey         uuid.UUID `json:"-"`
}

type resendRecoveryContactRequest struct {
	ExpectedPendingVersion uint64    `json:"expected_pending_version" validate:"required,min=1"`
	IdempotencyKey         uuid.UUID `json:"-"`
}

type cancelPendingRecoveryContactRequest struct {
	ExpectedPendingVersion uint64    `json:"expected_pending_version" validate:"required,min=1"`
	IdempotencyKey         uuid.UUID `json:"-"`
}

type removeRecoveryContactRequest struct {
	ExpectedCurrentVersion uint64    `json:"expected_current_version" validate:"required,min=1"`
	IdempotencyKey         uuid.UUID `json:"-"`
}
