package http

import dto "github.com/huynhanx03/judgify/internal/identity/contract"

func toOAuthLoginInspectionResponse(value *dto.OAuthLoginInspectionResponse) *oauthLoginInspectionResponse {
	if value == nil {
		return nil
	}
	r := &oauthLoginInspectionResponse{OnboardingRequired: value.OnboardingRequired, ExpiresAt: value.ExpiresAt, ServerTime: value.ServerTime, ProfileSuggestions: make([]oauthProfileSuggestionResponse, len(value.ProfileSuggestions))}
	for i, v := range value.ProfileSuggestions {
		r.ProfileSuggestions[i] = oauthProfileSuggestionResponse{Key: v.Key, Value: append([]byte(nil), v.Value...)}
	}
	return r
}
