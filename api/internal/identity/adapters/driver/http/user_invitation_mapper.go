package http

import (
	"github.com/google/uuid"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func toUserInvitationCommandRequest(value *userInvitationCommandRequest) *dto.UserInvitationCommandRequest {
	if value == nil {
		return nil
	}
	profileValues := make([]dto.ProfileValueInput, len(value.ProfileValues))
	for index, profileValue := range value.ProfileValues {
		profileValues[index] = dto.ProfileValueInput{DefinitionID: profileValue.DefinitionID, DefinitionRevisionID: profileValue.DefinitionRevisionID, Value: append([]byte(nil), profileValue.Value...)}
	}
	return &dto.UserInvitationCommandRequest{
		CommandID: value.CommandID, Username: value.Username, RecoveryEmail: value.RecoveryEmail,
		Roles: append([]string(nil), value.Roles...), Reason: value.Reason, ConfirmationToken: value.ConfirmationToken,
		ProfileSchemaEvidence: dto.ProfileSchemaEvidence{ProfileSchemaRevision: value.ProfileSchemaRevision, ProfileSchemaChecksum: value.ProfileSchemaChecksum, ProfileValues: profileValues},
	}
}

func toUserInvitationResendCommandRequest(value *userInvitationResendCommandRequest) (*dto.UserInvitationResendCommandRequest, error) {
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
	return &dto.UserInvitationResendCommandRequest{ID: id, CommandID: commandID, ExpectedUserVersion: value.ExpectedUserVersion, ExpectedInvitationVersion: value.ExpectedInvitationVersion, Reason: value.Reason, ConfirmationToken: value.ConfirmationToken}, nil
}

func toUserInvitationPreviewResponse(value *dto.UserInvitationPreviewResponse) *userInvitationPreviewResponse {
	if value == nil {
		return nil
	}
	result := &userInvitationPreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.Username = value.Preview.Username
	result.Preview.MaskedDestination = value.Preview.MaskedDestination
	result.Preview.Roles = append([]string(nil), value.Preview.Roles...)
	result.Preview.ProfileValueCount = value.Preview.ProfileValueCount
	return result
}
func toUserInvitationReceiptResponse(value *dto.UserInvitationReceipt) *userInvitationReceiptResponse {
	if value == nil {
		return nil
	}
	return &userInvitationReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), User: toUserLifecycleProjectionResponse(value.User), Roles: append([]string(nil), value.Roles...), ProfileValueCount: value.ProfileValueCount, InvitationExpiresAt: value.InvitationExpiresAt, DeliveryState: value.DeliveryState, CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
func toUserInvitationResendPreviewResponse(value *dto.UserInvitationResendPreviewResponse) *userInvitationResendPreviewResponse {
	if value == nil {
		return nil
	}
	result := &userInvitationResendPreviewResponse{ConfirmationToken: value.ConfirmationToken, ExpiresAt: value.ExpiresAt}
	result.Preview.User = toUserLifecycleProjectionResponse(value.Preview.User)
	result.Preview.MaskedDestination = value.Preview.MaskedDestination
	result.Preview.ExpectedInvitationVersion = value.Preview.ExpectedInvitationVersion
	result.Preview.ResultingInvitationVersion = value.Preview.ResultingInvitationVersion
	result.Preview.CurrentExpiresAt = value.Preview.CurrentExpiresAt
	return result
}
func toUserInvitationResendReceiptResponse(value *dto.UserInvitationResendReceipt) *userInvitationResendReceiptResponse {
	if value == nil {
		return nil
	}
	return &userInvitationResendReceiptResponse{CommandID: value.CommandID.String(), EventID: value.EventID.String(), User: toUserLifecycleProjectionResponse(value.User), InvitationVersion: value.InvitationVersion, InvitationExpiresAt: value.InvitationExpiresAt, DeliveryState: value.DeliveryState, CommittedAt: value.CommittedAt, IdempotentReplay: value.IdempotentReplay}
}
