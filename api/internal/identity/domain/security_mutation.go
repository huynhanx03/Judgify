package domain

import (
	"time"

	"github.com/google/uuid"
)

// SecurityMutationKind is the closed Identity security-event catalog. Adding
// a new kind requires an explicit journal mapping; callers cannot choose an
// arbitrary audit action, event type, status, or payload shape.
type SecurityMutationKind string

const (
	SecurityMutationUserRegistered                    SecurityMutationKind = "user_registered"
	SecurityMutationOAuthUserRegistered               SecurityMutationKind = "oauth_user_registered"
	SecurityMutationOAuthIdentityLinked               SecurityMutationKind = "oauth_identity_linked"
	SecurityMutationOAuthIdentityUnlinked             SecurityMutationKind = "oauth_identity_unlinked"
	SecurityMutationRecoveryContactVerified           SecurityMutationKind = "recovery_contact_verified"
	SecurityMutationRecoveryContactReplacementStarted SecurityMutationKind = "recovery_contact_replacement_started"
	SecurityMutationRecoveryContactVerificationResent SecurityMutationKind = "recovery_contact_verification_resent"
	SecurityMutationRecoveryContactPendingCancelled   SecurityMutationKind = "recovery_contact_pending_cancelled"
	SecurityMutationRecoveryContactRemoved            SecurityMutationKind = "recovery_contact_removed"
	SecurityMutationPasswordResetConsumed             SecurityMutationKind = "password_reset_consumed"
	SecurityMutationPasswordChanged                   SecurityMutationKind = "password_changed"
	SecurityMutationSessionRevoked                    SecurityMutationKind = "session_revoked"
	SecurityMutationSessionsRevokedAll                SecurityMutationKind = "sessions_revoked_all"
)

// SecurityMutation contains only identifiers and monotonic state facts. It
// deliberately has no free-form map or string field where an address,
// username, credential, ciphertext, hash, or password could be smuggled into
// audit/outbox persistence.
type SecurityMutation struct {
	ID                  uuid.UUID
	Kind                SecurityMutationKind
	UserID              uuid.UUID
	SessionID           *uuid.UUID
	TargetSessionID     *uuid.UUID
	ContactID           *uuid.UUID
	IntentID            *uuid.UUID
	FederatedIdentityID *uuid.UUID
	Provider            string
	Version             uint64
	SecurityVersion     uint64
	OccurredAt          time.Time
}
