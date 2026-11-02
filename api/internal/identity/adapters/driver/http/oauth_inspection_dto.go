package http

import (
	"encoding/json"
	"time"
)

type oauthProfileSuggestionResponse struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}
type oauthLoginInspectionResponse struct {
	OnboardingRequired bool                             `json:"onboarding_required"`
	ProfileSuggestions []oauthProfileSuggestionResponse `json:"profile_suggestions,omitempty"`
	ExpiresAt          time.Time                        `json:"expires_at"`
	ServerTime         time.Time                        `json:"server_time"`
}
