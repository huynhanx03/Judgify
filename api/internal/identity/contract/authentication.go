package contract

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

// RegisterRequest represents the payload for user registration.
type RegisterRequest struct {
	Username      string
	Password      string
	RecoveryEmail string
	TraitOfferSelectionRequest
	ProfileSchemaEvidence

	// Populated exclusively by the HTTP adapter from bounded headers and
	// connection metadata. They are never accepted from JSON.
	PreSessionID   uuid.UUID
	IdempotencyKey uuid.UUID
	UserAgent      string
	IPHash         []byte
}

// TraitOfferSelectionRequest is the shared browser-selected onboarding
// evidence used by password registration and first-login OAuth completion.
type TraitOfferSelectionRequest struct {
	OfferID           uuid.UUID
	OfferSequence     int64
	CatalogRevisionID uuid.UUID
	CatalogChecksum   string
	OfferChecksum     string
	RootBoneID        uuid.UUID
	TalentIDs         []uuid.UUID
}

// RegisterResponse represents the response for user registration.
type RegisterResponse struct {
	Success               bool
	SessionID             uuid.UUID
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

// LoginRequest represents the payload for user login.
type LoginRequest struct {
	Username string
	Password string

	PreSessionID   uuid.UUID
	IdempotencyKey uuid.UUID
	UserAgent      string
	IPHash         []byte
}

// LoginResponse represents the response for user login.
type LoginResponse struct {
	SessionID             uuid.UUID
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

// ChangePasswordRequest represents the payload for changing password.
type ChangePasswordRequest struct {
	CurrentPassword string
	NewPassword     string

	// Bound by the authenticated transport; never accepted from JSON.
	SessionID uuid.UUID
}

// ChangePasswordResponse represents the response for changing password.
type ChangePasswordResponse struct {
	Success bool
}

// RefreshTokenRequest represents the payload for refreshing access token.
type RefreshTokenRequest struct {
	RefreshToken   string
	IdempotencyKey uuid.UUID
}

// RefreshTokenResponse represents the response for refreshing access token.
type RefreshTokenResponse struct {
	SessionID             uuid.UUID
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type LogoutResponse struct {
	Success bool
}

// OAuthStartRequest represents the payload for building a provider authorization URL.
type OAuthStartRequest struct {
	Provider     string
	Purpose      entity.OAuthPurpose
	PreSessionID uuid.UUID
	SessionID    uuid.UUID
}

// OAuthStartResponse returns the provider authorization URL.
type OAuthStartResponse struct {
	AuthURL string
}

// OAuthCallbackRequest represents the payload for OAuth provider callback.
type OAuthCallbackRequest struct {
	Code     string
	State    string
	Provider string
}

// OAuthCallbackResponse carries only the opaque artifact needed by a later
// finalize call. Callback never creates an account, link, or session.
type OAuthCallbackResponse struct {
	CompletionTicket string
	TicketID         uuid.UUID
	Purpose          entity.OAuthPurpose
	ExpiresAt        time.Time
}

type OAuthLoginInspectionRequest struct {
	CompletionTicket string
	PreSessionID     uuid.UUID
}

type OAuthLoginInspectionResponse struct {
	OnboardingRequired bool
	ProfileSuggestions []ProfileValueSuggestion
	ExpiresAt          time.Time
	ServerTime         time.Time
}

type ProfileValueSuggestion struct {
	Key   string
	Value json.RawMessage
}

type OAuthOnboardingRequest struct {
	TraitOfferSelectionRequest
	ProfileSchemaEvidence
}

type OAuthFinalizeLoginRequest struct {
	CompletionTicket string
	PreSessionID     uuid.UUID
	IdempotencyKey   uuid.UUID
	UserAgent        string
	IPHash           []byte
	Onboarding       *OAuthOnboardingRequest
}

type OAuthFinalizeLoginResponse struct {
	SessionID             uuid.UUID
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type OAuthFinalizeLinkRequest struct {
	CompletionTicket string
	UserID           uuid.UUID
	SessionID        uuid.UUID
}

type OAuthFinalizeLinkResponse struct {
	Success bool
}

type OAuthIdentityResponse struct {
	Provider            string
	DisplayName         *string
	AvatarURL           *string
	LastAuthenticatedAt *time.Time
	LinkedAt            time.Time
}

type OAuthIdentityListResponse struct {
	Items []OAuthIdentityResponse
}

type OAuthProviderRequest struct {
	Provider string
}

// ForgotPasswordRequest represents the payload for forgot password.
type ForgotPasswordRequest struct {
	Username string
	IPHash   []byte
}

const ForgotPasswordStatusAccepted = "accepted"

// ForgotPasswordResponse represents the response for forgot password.
type ForgotPasswordResponse struct {
	Status string
}

// ResetPasswordRequest represents the payload for resetting password.
type ResetPasswordRequest struct {
	Token       string
	NewPassword string
}

// ResetPasswordResponse represents the response for resetting password.
type ResetPasswordResponse struct {
	Success bool
}
