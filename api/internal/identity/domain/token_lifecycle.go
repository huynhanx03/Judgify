package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionIssuancePurpose is a closed command identity used by recoverable
// session establishment. It mirrors the database enum without leaking Ent
// types into the core.
type SessionIssuancePurpose string

const (
	SessionIssuancePasswordLogin        SessionIssuancePurpose = "password_login"
	SessionIssuancePasswordRegistration SessionIssuancePurpose = "password_registration"
	SessionIssuanceOAuthLogin           SessionIssuancePurpose = "oauth_login"
	SessionIssuanceOAuthRegistration    SessionIssuancePurpose = "oauth_registration"
)

// SessionIssuanceReceipt stores only a keyed request fingerprint and
// short-lived authenticated ciphertext. Raw credentials never cross the
// repository boundary.
type SessionIssuanceReceipt struct {
	ID                 uuid.UUID
	Purpose            SessionIssuancePurpose
	RequestOrTicketID  *uuid.UUID
	UserID             *uuid.UUID
	PreSessionID       *uuid.UUID
	RequestFingerprint []byte `json:"-"`
	FingerprintKeyID   *string
	IdempotencyKey     uuid.UUID
	SessionID          uuid.UUID
	RefreshTokenID     uuid.UUID
	TraitOfferID       *uuid.UUID
	OfferChecksum      []byte `json:"-"`
	RecoveryCiphertext []byte `json:"-"`
	KeyID              string
	ExpiresAt          time.Time
	RecoveredAt        *time.Time
	ErasedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RefreshReceipt makes a successful single-use rotation exactly recoverable
// when the HTTP response is lost. Its ciphertext retention is deliberately
// much shorter than the refresh credential lifetime.
type RefreshReceipt struct {
	ID                 uuid.UUID
	SessionID          uuid.UUID
	PresentedTokenID   uuid.UUID
	IdempotencyKey     uuid.UUID
	ReplacementTokenID uuid.UUID
	RecoveryCiphertext []byte `json:"-"`
	KeyID              string
	ExpiresAt          time.Time
	RecoveredAt        *time.Time
	ErasedAt           *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SessionCredentials is the transport-neutral result of issuance or
// rotation. RefreshToken is intentionally marked secret and HTTP adapters must
// place it in an HttpOnly cookie rather than serializing it.
type SessionCredentials struct {
	SessionID             uuid.UUID
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string `json:"-"`
	RefreshTokenExpiresAt time.Time
}

// RefreshRotationOutcome distinguishes a committed replay containment action
// from an ordinary invalid credential. Returning replay as an error from the
// repository would roll back the revocation transaction.
type RefreshRotationOutcome uint8

const (
	RefreshRotationIssued RefreshRotationOutcome = iota + 1
	RefreshRotationRecovered
	RefreshRotationReplayContained
)

type RefreshRotationResult struct {
	Outcome            RefreshRotationOutcome
	ReceiptID          uuid.UUID
	ReplacementTokenID uuid.UUID
	SessionID          uuid.UUID
	UserID             uuid.UUID
	Username           string
	SecurityVersion    uint64
	AuthenticatedAt    time.Time
	AbsoluteExpiresAt  time.Time
	IdleExpiresAt      time.Time
	RefreshExpiresAt   time.Time
	RecoveryCiphertext []byte `json:"-"`
}
