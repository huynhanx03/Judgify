package schema

// Identity security values are closed at the storage boundary. Keeping them
// here avoids subtly different spellings across schemas, migrations, and
// invariant tests.
const (
	userStatusInvited   = "invited"
	userStatusActive    = "active"
	userStatusSuspended = "suspended"
	userStatusDeleted   = "deleted"

	sessionStatusActive  = "active"
	sessionStatusRevoked = "revoked"
	sessionStatusExpired = "expired"

	refreshStatusIssued   = "issued"
	refreshStatusConsumed = "consumed"
	refreshStatusRevoked  = "revoked"

	oneTimeStatusIssued     = "issued"
	oneTimeStatusConsumed   = "consumed"
	oneTimeStatusSuperseded = "superseded"
	oneTimeStatusExpired    = "expired"

	contactStatusPending       = "pending"
	contactStatusVerified      = "verified"
	contactStatusRevoked       = "revoked"
	contactPurposeRegistration = "registration"
	contactPurposeReplacement  = "replacement"
	contactPurposeInvitation   = "invitation"

	deliveryStatusReady     = "ready"
	deliveryStatusLeased    = "leased"
	deliveryStatusSent      = "sent"
	deliveryStatusDead      = "dead"
	deliveryStatusCancelled = "cancelled"

	oauthFlowStatusPending  = "pending"
	oauthFlowStatusConsumed = "consumed"
	oauthFlowStatusExpired  = "expired"
)
