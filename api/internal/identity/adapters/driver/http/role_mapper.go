package http

import (
	"fmt"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func parseRoleID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid role id")
	}
	return id, nil
}

func parseRoleCommandID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid role command id")
	}
	return id, nil
}

func toCreateRoleRequest(value *createRoleRequest) (*dto.CreateRoleRequest, error) {
	if value == nil {
		return nil, nil
	}
	commandID, err := parseRoleCommandID(value.CommandID)
	if err != nil {
		return nil, err
	}
	return &dto.CreateRoleRequest{CommandID: commandID, ExpectedVersion: value.ExpectedVersion, Reason: value.Reason, Key: value.Key, Name: value.Name, Description: value.Description}, nil
}

func toUpdateRoleRequest(value *roleCommandRequest) (*dto.UpdateRoleRequest, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseRoleID(value.ID)
	if err != nil {
		return nil, err
	}
	commandID, err := parseRoleCommandID(value.CommandID)
	if err != nil {
		return nil, err
	}
	return &dto.UpdateRoleRequest{ID: id, CommandID: commandID, ExpectedVersion: value.ExpectedVersion, Reason: value.Reason, Name: value.Name, Description: value.Description, ConfirmationToken: value.ConfirmationToken}, nil
}

func toGetRoleRequest(value *getRoleRequest) (*dto.GetRoleRequest, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseRoleID(value.ID)
	if err != nil {
		return nil, err
	}
	return &dto.GetRoleRequest{ID: id}, nil
}

func toRoleResponse(value *dto.RoleResponse) *roleResponse {
	if value == nil {
		return nil
	}
	return &roleResponse{ID: value.ID.String(), Key: value.Key, Name: value.Name, Description: value.Description, IsSystem: value.IsSystem, IsProtected: value.IsProtected, Status: value.Status, Version: value.Version}
}

func toRoleResponses(values []*dto.RoleResponse) []*roleResponse {
	result := make([]*roleResponse, 0, len(values))
	for _, value := range values {
		result = append(result, toRoleResponse(value))
	}
	return result
}

func toRolePage(value *d.Paginated[*dto.RoleResponse]) *d.Paginated[*roleResponse] {
	if value == nil {
		return nil
	}
	result := &d.Paginated[*roleResponse]{Pagination: value.Pagination}
	if value.Records != nil {
		records := toRoleResponses(*value.Records)
		result.Records = &records
	}
	return result
}

func toRoleMetadataPreviewResponse(value *dto.RoleMetadataPreviewResponse) *roleMetadataPreviewResponse {
	if value == nil {
		return nil
	}
	result := &roleMetadataPreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.Action = value.Preview.Action
	result.Preview.RoleID = value.Preview.RoleID.String()
	result.Preview.ExpectedVersion = value.Preview.ExpectedVersion
	result.Preview.ResultingVersion = value.Preview.ResultingVersion
	result.Preview.Before = *toRoleResponse(&value.Preview.Before)
	result.Preview.After = *toRoleResponse(&value.Preview.After)
	return result
}

func toRoleMutationReceiptResponse(value *dto.RoleMutationReceipt) *roleMutationReceiptResponse {
	if value == nil {
		return nil
	}
	return &roleMutationReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), Action: value.Action, Role: *toRoleResponse(&value.Role), CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
