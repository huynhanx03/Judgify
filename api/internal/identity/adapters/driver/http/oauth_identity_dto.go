package http

import "time"

type oauthProviderRequest struct {
	Provider string `uri:"provider" validate:"required,max=64"`
}
type oauthIdentityResponse struct {
	Provider            string     `json:"provider"`
	DisplayName         *string    `json:"display_name,omitempty"`
	AvatarURL           *string    `json:"avatar_url,omitempty"`
	LastAuthenticatedAt *time.Time `json:"last_authenticated_at,omitempty"`
	LinkedAt            time.Time  `json:"linked_at"`
}
type oauthIdentityListResponse struct {
	Items []oauthIdentityResponse `json:"items"`
}
type oauthUnlinkResponse struct {
	Success bool `json:"success"`
}
