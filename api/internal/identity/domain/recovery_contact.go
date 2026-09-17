package domain

import (
	"time"

	"github.com/google/uuid"
)

// KeyedTokenHash identifies one bounded keyring candidate without exposing the
// raw one-time credential. Hash is always a 32-byte HMAC and must never be
// serialized or logged.
type KeyedTokenHash struct {
	KeyID string
	Hash  []byte `json:"-"`
}

type RecoveryContactStatus string

const (
	RecoveryContactStatusPending  RecoveryContactStatus = "pending"
	RecoveryContactStatusVerified RecoveryContactStatus = "verified"
	RecoveryContactStatusRevoked  RecoveryContactStatus = "revoked"
)

type RecoveryContactPurpose string

const (
	RecoveryContactPurposeRegistration RecoveryContactPurpose = "registration"
	RecoveryContactPurposeReplacement  RecoveryContactPurpose = "replacement"
	RecoveryContactPurposeInvitation   RecoveryContactPurpose = "invitation"
)

// RecoveryContactDeliveryState is the deliberately small, provider-neutral
// delivery projection exposed to the owning user. Provider identifiers and
// retry diagnostics stay inside the delivery subsystem.
type RecoveryContactDeliveryState string

const (
	RecoveryContactDeliveryQueued RecoveryContactDeliveryState = "queued"
	RecoveryContactDeliverySent   RecoveryContactDeliveryState = "sent"
	RecoveryContactDeliveryFailed RecoveryContactDeliveryState = "failed"
)

// RecoveryContact is the protected domain projection. AddressCiphertext and
// blind indexes never cross the service/adapter boundary into transport DTOs.
type RecoveryContact struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Kind              string
	Purpose           RecoveryContactPurpose
	Status            RecoveryContactStatus
	Version           uint64
	AddressCiphertext []byte `json:"-"`
	KeyID             string
	VerifiedAt        *time.Time
	RevokedAt         *time.Time
	ReplacedContactID *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// RecoveryContactVerificationResult contains only non-secret identifiers from
// the committed activation. It is safe for structured logs but is not exposed
// by the public HTTP response.
type RecoveryContactVerificationResult struct {
	UserID    uuid.UUID
	ContactID uuid.UUID
	Purpose   RecoveryContactPurpose
	Version   uint64
}

// RecoveryContactView is safe to cross the HTTP boundary. It contains only a
// masked destination and optimistic state, never ciphertext or blind indexes.
type RecoveryContactView struct {
	Status        RecoveryContactStatus
	Purpose       RecoveryContactPurpose
	MaskedAddress string
	Version       uint64
}

type PendingRecoveryContactView struct {
	RecoveryContactView
	VerificationExpiresAt time.Time
	ResendAvailableAt     time.Time
	DeliveryState         RecoveryContactDeliveryState
}

type RecoveryContactProjection struct {
	Current              *RecoveryContactView
	Pending              *PendingRecoveryContactView
	ReauthenticatedUntil *time.Time
	CanRemove            bool
}
