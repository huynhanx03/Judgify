package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toOAuthStartRequest(value *oauthStartRequest) *dto.OAuthStartRequest {
	if value == nil {
		return nil
	}
	return &dto.OAuthStartRequest{Provider: value.Provider}
}
func toOAuthStartResponse(value *dto.OAuthStartResponse) *oauthStartResponse {
	if value == nil {
		return nil
	}
	return &oauthStartResponse{AuthURL: value.AuthURL}
}
