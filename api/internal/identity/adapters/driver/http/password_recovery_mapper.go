package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toForgotPasswordRequest(value *forgotPasswordRequest) *dto.ForgotPasswordRequest {
	if value == nil {
		return nil
	}
	return &dto.ForgotPasswordRequest{Username: value.Username, IPHash: append([]byte(nil), value.IPHash...)}
}
func toResetPasswordRequest(value *resetPasswordRequest) *dto.ResetPasswordRequest {
	if value == nil {
		return nil
	}
	return &dto.ResetPasswordRequest{Token: value.Token, NewPassword: value.NewPassword}
}
func toForgotPasswordResponse(value *dto.ForgotPasswordResponse) *forgotPasswordResponse {
	if value == nil {
		return nil
	}
	return &forgotPasswordResponse{Status: value.Status}
}
func toResetPasswordResponse(value *dto.ResetPasswordResponse) *resetPasswordResponse {
	if value == nil {
		return nil
	}
	return &resetPasswordResponse{Success: value.Success}
}
