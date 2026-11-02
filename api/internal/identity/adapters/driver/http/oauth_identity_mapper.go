package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toOAuthIdentityListResponse(value *dto.OAuthIdentityListResponse) *oauthIdentityListResponse {
	if value == nil {
		return nil
	}
	r := &oauthIdentityListResponse{Items: make([]oauthIdentityResponse, len(value.Items))}
	for i, v := range value.Items {
		r.Items[i] = oauthIdentityResponse{Provider: v.Provider, DisplayName: v.DisplayName, AvatarURL: v.AvatarURL, LastAuthenticatedAt: v.LastAuthenticatedAt, LinkedAt: v.LinkedAt}
	}
	return r
}
func toOAuthUnlinkResponse(value *dto.OAuthFinalizeLinkResponse) *oauthUnlinkResponse {
	if value == nil {
		return nil
	}
	return &oauthUnlinkResponse{Success: value.Success}
}
func toOAuthProviderRequest(value *oauthProviderRequest) *dto.OAuthProviderRequest {
	if value == nil {
		return nil
	}
	return &dto.OAuthProviderRequest{Provider: value.Provider}
}
