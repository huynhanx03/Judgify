package http

import (
	"github.com/google/uuid"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func toUserLifecycleCommand(value *userLifecycleCommandRequest) (*dto.UserLifecycleCommandRequest, error) {
	if value == nil {
		return nil, nil
	}
	id, err := uuid.Parse(value.ID)
	if err != nil || id == uuid.Nil {
		return nil, err
	}
	commandID, err := uuid.Parse(value.CommandID)
	if err != nil || commandID == uuid.Nil {
		return nil, err
	}
	return &dto.UserLifecycleCommandRequest{ID: id, CommandID: commandID, ExpectedVersion: value.ExpectedVersion, Reason: value.Reason, ConfirmationToken: value.ConfirmationToken}, nil
}

func toUserLifecycleProjectionResponse(value dto.UserLifecycleProjection) userLifecycleProjectionResponse {
	return userLifecycleProjectionResponse{ID: value.ID.String(), Username: value.Username, Status: value.Status, Version: value.Version, SuspendedAt: value.SuspendedAt}
}
func toUserLifecyclePreviewResponse(value *dto.UserLifecyclePreviewResponse) *userLifecyclePreviewResponse {
	if value == nil {
		return nil
	}
	result := &userLifecyclePreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.Action = value.Preview.Action
	result.Preview.UserID = value.Preview.UserID.String()
	result.Preview.ExpectedVersion = value.Preview.ExpectedVersion
	result.Preview.ResultingVersion = value.Preview.ResultingVersion
	result.Preview.Before = toUserLifecycleProjectionResponse(value.Preview.Before)
	result.Preview.After = toUserLifecycleProjectionResponse(value.Preview.After)
	return result
}
func toUserLifecycleReceiptResponse(value *dto.UserLifecycleReceipt) *userLifecycleReceiptResponse {
	if value == nil {
		return nil
	}
	return &userLifecycleReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), Action: value.Action, User: toUserLifecycleProjectionResponse(value.User), CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
