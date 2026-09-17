package http

type forgotPasswordRequest struct {
	Username string `json:"username" validate:"required,max=50"`
	IPHash   []byte
}
type resetPasswordRequest struct {
	Token       string `json:"token" validate:"required,max=4096"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=256"`
}
type forgotPasswordResponse struct {
	Status string `json:"status"`
}
type resetPasswordResponse struct {
	Success bool `json:"success"`
}
