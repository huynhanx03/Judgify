package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toActivateUserInvitationRequest(value *activateUserInvitationRequest) *dto.ActivateUserInvitationRequest {
	if value == nil {
		return nil
	}
	return &dto.ActivateUserInvitationRequest{Token: value.Token, NewPassword: value.NewPassword}
}

func toActivateUserInvitationResponse(value *dto.ActivateUserInvitationResponse) *activateUserInvitationResponse {
	if value == nil {
		return nil
	}
	return &activateUserInvitationResponse{Success: value.Success}
}
