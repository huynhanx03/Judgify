package http

type oauthStartRequest struct {
	Provider string `uri:"provider" validate:"required,max=64"`
}
type oauthStartResponse struct {
	AuthURL string `json:"auth_url"`
}
