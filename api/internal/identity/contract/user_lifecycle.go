package contract

import (
	"time"

	"github.com/google/uuid"
)

// UserLifecycleCommandRequest is shared by preview and apply. The transport
// additionally requires CommandID to exactly match the canonical
// Idempotency-Key header; actor and session identities never come from JSON.
type UserLifecycleCommandRequest struct {
	ID                uuid.UUID
	CommandID         uuid.UUID
	ExpectedVersion   int64
	Reason            string
	ConfirmationToken string
}

type UserLifecycleProjection struct {
	ID          uuid.UUID
	Username    string
	Status      string
	Version     int64
	SuspendedAt *time.Time
}

type UserLifecyclePreview struct {
	Action           string
	UserID           uuid.UUID
	ExpectedVersion  int64
	ResultingVersion int64
	Before           UserLifecycleProjection
	After            UserLifecycleProjection
}

type UserLifecyclePreviewResponse struct {
	Preview           UserLifecyclePreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

type UserLifecycleReceipt struct {
	CommandID        uuid.UUID
	EventID          uuid.UUID
	Action           string
	User             UserLifecycleProjection
	CommittedAt      time.Time
	IdempotentReplay bool
}
