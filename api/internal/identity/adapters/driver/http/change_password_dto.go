package http

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,max=256"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=256"`
}
type changePasswordResponse struct {
	Success bool `json:"success"`
}
