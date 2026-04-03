package dto

// RegisterRequest represents the payload for user registration.
type RegisterRequest struct {
	Username   string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password   string `json:"password" validate:"required,min=8"`
	FirstName  string `json:"first_name" validate:"required,max=100"`
	LastName   string `json:"last_name" validate:"required,max=100"`
	Gender     int    `json:"gender" validate:"oneof=0 1 2"` // 0: male, 1: female, 2: other
	Birthday   string `json:"birthday" validate:"required,datetime=2006-01-02"`
	RootBoneID int   `json:"root_bone_id" validate:"required"`  // Selected root bone trait ID
	TalentIDs  []int `json:"talent_ids" validate:"required,len=3"` // Selected talent trait IDs (exactly 3)
}

// RegisterResponse represents the response for user registration.
type RegisterResponse struct {
	Success bool `json:"success"`
}

// LoginRequest represents the payload for user login.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the response for user login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ChangePasswordRequest represents the payload for changing password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ChangePasswordResponse represents the response for changing password.
type ChangePasswordResponse struct {
	Success bool `json:"success"`
}

// RefreshTokenRequest represents the payload for refreshing access token.
type RefreshTokenRequest struct {
}

// RefreshTokenResponse represents the response for refreshing access token.
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// OAuthCallbackRequest represents the payload for OAuth provider callback.
type OAuthCallbackRequest struct {
	Code     string `json:"code" validate:"required"`
	Provider string `uri:"provider"`
}

// OAuthCallbackResponse represents the response for OAuth provider callback.
type OAuthCallbackResponse struct {
	// If the user already exists, tokens are returned directly.
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`

	// If the user is new, a temporary token is returned for registration.
	RequiresRegistration bool   `json:"requires_registration"`
	OAuthToken           string `json:"oauth_token,omitempty"`
}

// OAuthRegisterRequest represents the payload for completing OAuth registration.
type OAuthRegisterRequest struct {
	OAuthToken string `json:"oauth_token" validate:"required"`
	Username   string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password   string `json:"password" validate:"required,min=8"`
	FirstName  string `json:"first_name" validate:"required,max=100"`
	LastName   string `json:"last_name" validate:"required,max=100"`
	Gender     int    `json:"gender" validate:"oneof=0 1 2"`
	Birthday   string `json:"birthday" validate:"required,datetime=2006-01-02"`
	Provider   string `uri:"provider"`
}

// OAuthLinkRequest represents the payload for linking an OAuth provider to an existing account.
type OAuthLinkRequest struct {
	Code     string `json:"code" validate:"required"`
	Provider string `uri:"provider"`
}

// OAuthLinkResponse represents the response for linking an OAuth provider.
type OAuthLinkResponse struct {
	Success bool `json:"success"`
}

// ForgotPasswordRequest represents the payload for forgot password.
type ForgotPasswordRequest struct {
	Username string `json:"username" validate:"required"`
}

// ForgotPasswordResponse represents the response for forgot password.
type ForgotPasswordResponse struct {
	Message string `json:"message"`
}

// ResetPasswordRequest represents the payload for resetting password.
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ResetPasswordResponse represents the response for resetting password.
type ResetPasswordResponse struct {
	Success bool `json:"success"`
}
