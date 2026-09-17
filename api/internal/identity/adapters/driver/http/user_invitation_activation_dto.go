package http

type activateUserInvitationRequest struct {
	Token       string `json:"token" validate:"required,max=4096"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=256"`
}

type activateUserInvitationResponse struct {
	Success bool `json:"success"`
}
