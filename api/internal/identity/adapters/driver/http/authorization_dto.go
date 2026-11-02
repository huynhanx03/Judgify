package http

import "time"

type policyRuleRequest struct {
	RoleKey  string `json:"role_key,omitempty"`
	Resource string `json:"resource" validate:"required"`
	Action   string `json:"action" validate:"required"`
}

type rolePoliciesCommandRequest struct {
	ID                string              `uri:"id" validate:"required"`
	CommandID         string              `json:"command_id" validate:"required"`
	ExpectedRevision  int64               `json:"expected_revision" validate:"required,gt=0"`
	Reason            string              `json:"reason" validate:"required,min=3,max=500"`
	Rules             []policyRuleRequest `json:"rules" validate:"max=256,dive"`
	ConfirmationToken string              `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type rolePoliciesUpdateRequest struct {
	ID               string              `uri:"id" validate:"required"`
	ExpectedRevision int64               `json:"expected_revision" validate:"required,gt=0"`
	Reason           string              `json:"reason" validate:"required,min=3,max=500"`
	Rules            []policyRuleRequest `json:"rules" validate:"max=256,dive"`
}

type userRolesCommandRequest struct {
	UserID            string   `uri:"id" validate:"required"`
	CommandID         string   `json:"command_id" validate:"required"`
	ExpectedRevision  int64    `json:"expected_revision" validate:"required,gt=0"`
	Reason            string   `json:"reason" validate:"required,min=3,max=500"`
	Roles             []string `json:"roles" validate:"required,min=1,max=64,dive,min=2,max=50"`
	ConfirmationToken string   `json:"confirmation_token,omitempty" validate:"omitempty,max=2048"`
}

type policyRuleResponse struct {
	RoleKey  string `json:"role_key,omitempty"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}
type rolePolicySnapshotResponse struct {
	Revision int64                `json:"revision"`
	Rules    []policyRuleResponse `json:"rules"`
}
type rolePolicyPreviewResponse struct {
	Preview struct {
		RoleKey           string                     `json:"role_key"`
		ExpectedRevision  int64                      `json:"expected_revision"`
		ResultingRevision int64                      `json:"resulting_revision"`
		Before            rolePolicySnapshotResponse `json:"before"`
		After             rolePolicySnapshotResponse `json:"after"`
		Added             []policyRuleResponse       `json:"added"`
		Removed           []policyRuleResponse       `json:"removed"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}
type rolePolicyMutationReceiptResponse struct {
	CommandID        string                     `json:"command_id"`
	EventID          string                     `json:"event_id"`
	RoleKey          string                     `json:"role_key"`
	Snapshot         rolePolicySnapshotResponse `json:"snapshot"`
	CommittedAt      time.Time                  `json:"committed_at"`
	IdempotentReplay bool                       `json:"idempotent_replay"`
}
type userRoleSnapshotResponse struct {
	Revision int64    `json:"revision"`
	Roles    []string `json:"roles"`
}
type userRolePreviewResponse struct {
	Preview struct {
		UserID            string                   `json:"user_id"`
		ExpectedRevision  int64                    `json:"expected_revision"`
		ResultingRevision int64                    `json:"resulting_revision"`
		Before            userRoleSnapshotResponse `json:"before"`
		After             userRoleSnapshotResponse `json:"after"`
		Added             []string                 `json:"added"`
		Removed           []string                 `json:"removed"`
	} `json:"preview"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresAt         time.Time `json:"expires_at"`
}
type userRoleMutationReceiptResponse struct {
	CommandID        string                   `json:"command_id"`
	EventID          string                   `json:"event_id"`
	UserID           string                   `json:"user_id"`
	Snapshot         userRoleSnapshotResponse `json:"snapshot"`
	CommittedAt      time.Time                `json:"committed_at"`
	IdempotentReplay bool                     `json:"idempotent_replay"`
}
type authorizationCatalogActionResponse struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}
type authorizationCatalogResourceResponse struct {
	Key         string                               `json:"key"`
	Label       string                               `json:"label"`
	Description string                               `json:"description"`
	Actions     []authorizationCatalogActionResponse `json:"actions"`
}
type authorizationCatalogResponse struct {
	Revision  int64                                  `json:"revision"`
	Resources []authorizationCatalogResourceResponse `json:"resources"`
}
type capabilityGrantResponse struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}
type effectiveCapabilitiesResponse struct {
	Revision     int64                     `json:"revision"`
	Capabilities []capabilityGrantResponse `json:"capabilities"`
}
