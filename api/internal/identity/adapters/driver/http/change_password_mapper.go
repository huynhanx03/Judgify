package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toChangePasswordRequest(value *changePasswordRequest) *dto.ChangePasswordRequest {
	if value == nil {
		return nil
	}
	return &dto.ChangePasswordRequest{CurrentPassword: value.CurrentPassword, NewPassword: value.NewPassword}
}
func toChangePasswordResponse(value *dto.ChangePasswordResponse) *changePasswordResponse {
	if value == nil {
		return nil
	}
	return &changePasswordResponse{Success: value.Success}
}
