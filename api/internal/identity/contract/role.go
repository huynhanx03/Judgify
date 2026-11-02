package contract

import (
	"time"

	"github.com/google/uuid"
)

// CreateRoleRequest represents request to create a role.
type CreateRoleRequest struct {
	CommandID       uuid.UUID
	ExpectedVersion int64
	Reason          string
	Key             string
	Name            string
	Description     string
}

// UpdateRoleRequest represents request to update a role.
type UpdateRoleRequest struct {
	ID                uuid.UUID
	CommandID         uuid.UUID
	ExpectedVersion   int64
	Reason            string
	Name              *string
	Description       *string
	ConfirmationToken string
}

// GetRoleRequest represents request to get a role by ID.
type GetRoleRequest struct {
	ID uuid.UUID
}

// ArchiveRoleRequest is intentionally identical to UpdateRoleRequest: archive
// requires the same command identity, optimistic version and confirmation.
type ArchiveRoleRequest = UpdateRoleRequest

// RoleMetadataPreview is the exact role projection reviewed before a mutable
// metadata command is applied. Keys and system ownership are server-owned.
type RoleMetadataPreview struct {
	Action           string
	RoleID           uuid.UUID
	ExpectedVersion  int64
	ResultingVersion int64
	Before           RoleResponse
	After            RoleResponse
}

type RoleMetadataPreviewResponse struct {
	Preview           RoleMetadataPreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

// RoleMutationReceipt is an immutable projection of the outbox row whose ID
// is the supplied command ID. It makes ambiguous HTTP delivery replay-safe.
type RoleMutationReceipt struct {
	CommandID        uuid.UUID
	EventID          uuid.UUID
	Action           string
	Role             RoleResponse
	CommittedAt      time.Time
	IdempotentReplay bool
}

// RoleResponse represents role data in API response.
type RoleResponse struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	IsSystem    bool
	IsProtected bool
	Status      string
	Version     int64
}
