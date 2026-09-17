package http

import (
	"fmt"
	"github.com/google/uuid"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func parseAuthorizationID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid authorization id")
	}
	return id, nil
}
func toPolicyRules(values []policyRuleRequest) []dto.PolicyRule {
	result := make([]dto.PolicyRule, len(values))
	for i, value := range values {
		result[i] = dto.PolicyRule{RoleKey: value.RoleKey, Resource: value.Resource, Action: value.Action}
	}
	return result
}
func toRolePoliciesCommand(value *rolePoliciesCommandRequest) (*dto.RolePoliciesCommandRequest, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseAuthorizationID(value.ID)
	if err != nil {
		return nil, err
	}
	commandID, err := parseAuthorizationID(value.CommandID)
	if err != nil {
		return nil, err
	}
	return &dto.RolePoliciesCommandRequest{ID: id, CommandID: commandID, ExpectedRevision: value.ExpectedRevision, Reason: value.Reason, Rules: toPolicyRules(value.Rules), ConfirmationToken: value.ConfirmationToken}, nil
}
func toRolePoliciesUpdate(value *rolePoliciesUpdateRequest) (*dto.RolePoliciesUpdateRequest, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseAuthorizationID(value.ID)
	if err != nil {
		return nil, err
	}
	return &dto.RolePoliciesUpdateRequest{ID: id, ExpectedRevision: value.ExpectedRevision, Reason: value.Reason, Rules: toPolicyRules(value.Rules)}, nil
}
func toUserRolesCommand(value *userRolesCommandRequest) (*dto.UserRolesCommandRequest, error) {
	if value == nil {
		return nil, nil
	}
	userID, err := parseAuthorizationID(value.UserID)
	if err != nil {
		return nil, err
	}
	commandID, err := parseAuthorizationID(value.CommandID)
	if err != nil {
		return nil, err
	}
	return &dto.UserRolesCommandRequest{UserID: userID, CommandID: commandID, ExpectedRevision: value.ExpectedRevision, Reason: value.Reason, Roles: append([]string(nil), value.Roles...), ConfirmationToken: value.ConfirmationToken}, nil
}
func toPolicyRuleResponses(values []dto.PolicyRule) []policyRuleResponse {
	result := make([]policyRuleResponse, len(values))
	for i, value := range values {
		result[i] = policyRuleResponse{RoleKey: value.RoleKey, Resource: value.Resource, Action: value.Action}
	}
	return result
}
func toRolePolicySnapshotResponse(value *dto.RolePolicySnapshot) *rolePolicySnapshotResponse {
	if value == nil {
		return nil
	}
	return &rolePolicySnapshotResponse{Revision: value.Revision, Rules: toPolicyRuleResponses(value.Rules)}
}
func toRolePolicyPreviewResponse(value *dto.RolePolicyPreviewResponse) *rolePolicyPreviewResponse {
	if value == nil {
		return nil
	}
	result := &rolePolicyPreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.RoleKey = value.Preview.RoleKey
	result.Preview.ExpectedRevision = value.Preview.ExpectedRevision
	result.Preview.ResultingRevision = value.Preview.ResultingRevision
	result.Preview.Before = *toRolePolicySnapshotResponse(&value.Preview.Before)
	result.Preview.After = *toRolePolicySnapshotResponse(&value.Preview.After)
	result.Preview.Added = toPolicyRuleResponses(value.Preview.Added)
	result.Preview.Removed = toPolicyRuleResponses(value.Preview.Removed)
	return result
}
func toRolePolicyMutationReceiptResponse(value *dto.RolePolicyMutationReceipt) *rolePolicyMutationReceiptResponse {
	if value == nil {
		return nil
	}
	return &rolePolicyMutationReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), RoleKey: value.RoleKey, Snapshot: *toRolePolicySnapshotResponse(&value.Snapshot), CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
func toUserRoleSnapshotResponse(value dto.UserRoleSnapshot) userRoleSnapshotResponse {
	return userRoleSnapshotResponse{Revision: value.Revision, Roles: append([]string(nil), value.Roles...)}
}
func toUserRolePreviewResponse(value *dto.UserRolePreviewResponse) *userRolePreviewResponse {
	if value == nil {
		return nil
	}
	result := &userRolePreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.UserID = value.Preview.UserID.String()
	result.Preview.ExpectedRevision = value.Preview.ExpectedRevision
	result.Preview.ResultingRevision = value.Preview.ResultingRevision
	result.Preview.Before = toUserRoleSnapshotResponse(value.Preview.Before)
	result.Preview.After = toUserRoleSnapshotResponse(value.Preview.After)
	result.Preview.Added = append([]string(nil), value.Preview.Added...)
	result.Preview.Removed = append([]string(nil), value.Preview.Removed...)
	return result
}
func toUserRoleMutationReceiptResponse(value *dto.UserRoleMutationReceipt) *userRoleMutationReceiptResponse {
	if value == nil {
		return nil
	}
	return &userRoleMutationReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), UserID: value.UserID.String(), Snapshot: toUserRoleSnapshotResponse(value.Snapshot), CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
func toAuthorizationCatalogResponse(value *dto.AuthorizationCatalogResponse) *authorizationCatalogResponse {
	if value == nil {
		return nil
	}
	result := &authorizationCatalogResponse{Revision: value.Revision, Resources: make([]authorizationCatalogResourceResponse, len(value.Resources))}
	for i, resource := range value.Resources {
		actions := make([]authorizationCatalogActionResponse, len(resource.Actions))
		for j, action := range resource.Actions {
			actions[j] = authorizationCatalogActionResponse{Key: action.Key, Label: action.Label, Description: action.Description}
		}
		result.Resources[i] = authorizationCatalogResourceResponse{Key: resource.Key, Label: resource.Label, Description: resource.Description, Actions: actions}
	}
	return result
}
func toEffectiveCapabilitiesResponse(value *dto.EffectiveCapabilitiesResponse) *effectiveCapabilitiesResponse {
	if value == nil {
		return nil
	}
	result := &effectiveCapabilitiesResponse{Revision: value.Revision, Capabilities: make([]capabilityGrantResponse, len(value.Capabilities))}
	for i, capability := range value.Capabilities {
		result.Capabilities[i] = capabilityGrantResponse{Resource: capability.Resource, Actions: append([]string(nil), capability.Actions...)}
	}
	return result
}
