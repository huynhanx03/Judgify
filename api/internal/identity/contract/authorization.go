package contract

import (
	"github.com/google/uuid"
	"time"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

// PolicyRule is one Casbin permission rule for a role.
type PolicyRule struct {
	RoleKey  string
	Resource string
	Action   string
}

// RolePoliciesUpdateRequest replaces all permission rules for a role route.
type RolePoliciesUpdateRequest struct {
	ID               uuid.UUID
	ExpectedRevision int64
	Reason           string
	Rules            []PolicyRule
}

// RolePoliciesCommandRequest is shared by preview and apply. Preview leaves
// ConfirmationToken empty; apply must echo the opaque token issued for this
// exact command. CommandID must match the canonical Idempotency-Key header.
type RolePoliciesCommandRequest struct {
	ID                uuid.UUID
	CommandID         uuid.UUID
	ExpectedRevision  int64
	Reason            string
	Rules             []PolicyRule
	ConfirmationToken string
}

// RolePolicySnapshot is one complete, revision-fenced role policy view.
// Clients must send Revision back as expected_revision when replacing Rules.
type RolePolicySnapshot struct {
	Revision int64
	Rules    []PolicyRule
}

type RolePolicyPreview struct {
	RoleKey           string
	ExpectedRevision  int64
	ResultingRevision int64
	Before            RolePolicySnapshot
	After             RolePolicySnapshot
	Added             []PolicyRule
	Removed           []PolicyRule
}

type RolePolicyPreviewResponse struct {
	Preview           RolePolicyPreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

// RolePolicyMutationReceipt is the exact durable result of one administrative
// replacement command. EventID equals CommandID by contract, allowing a lost
// HTTP response to be recovered without applying the mutation twice.
type RolePolicyMutationReceipt struct {
	CommandID        uuid.UUID
	EventID          uuid.UUID
	RoleKey          string
	Snapshot         RolePolicySnapshot
	CommittedAt      time.Time
	IdempotentReplay bool
}

// UserRolesCommandRequest is shared by reviewed preview/apply endpoints.
// CommandID must equal the canonical Idempotency-Key header and apply echoes
// the opaque token issued for the exact previewed request.
type UserRolesCommandRequest struct {
	UserID            uuid.UUID
	CommandID         uuid.UUID
	ExpectedRevision  int64
	Reason            string
	Roles             []string
	ConfirmationToken string
}

type UserRolePreview struct {
	UserID            uuid.UUID
	ExpectedRevision  int64
	ResultingRevision int64
	Before            UserRoleSnapshot
	After             UserRoleSnapshot
	Added             []string
	Removed           []string
}

type UserRolePreviewResponse struct {
	Preview           UserRolePreview
	ConfirmationToken string
	ExpiresAt         time.Time
}

type UserRoleMutationReceipt struct {
	CommandID        uuid.UUID
	EventID          uuid.UUID
	UserID           uuid.UUID
	Snapshot         UserRoleSnapshot
	CommittedAt      time.Time
	IdempotentReplay bool
}

// AuthorizationCatalogResponse is the code-owned capability catalog paired
// with the current persistence revision. Presentation copy is deploy-owned;
// persisted policy rows contain only stable keys.
type AuthorizationCatalogResponse struct {
	Revision  int64
	Resources []permissions.CatalogResource
}

// CapabilityGrant is one bounded, code-owned effective resource grant. Raw
// Casbin subjects, roles, policy rows, and wildcard values never cross this
// boundary.
type CapabilityGrant struct {
	Resource string
	Actions  []string
}

// EffectiveCapabilitiesResponse is the principal's complete capability view
// at one applied authorization revision.
type EffectiveCapabilitiesResponse struct {
	Revision     int64
	Capabilities []CapabilityGrant
}

// UserRolesRequest replaces all roles assigned to one user.
type UserRolesRequest struct {
	UserID uuid.UUID
	Roles  []string
}

// UserRoleSnapshot is the global authorization revision paired with one
// user's direct role assignments.
type UserRoleSnapshot struct {
	Revision int64
	Roles    []string
}
