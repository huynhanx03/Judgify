package domain

import (
	"time"

	"github.com/google/uuid"
)

// OAuthPurpose is a closed application catalog. Reauthentication is reserved
// here even though the current Ent schema persists only login and link flows.
type OAuthPurpose string

const (
	OAuthPurposeLogin  OAuthPurpose = "login"
	OAuthPurposeLink   OAuthPurpose = "link"
	OAuthPurposeReauth OAuthPurpose = "reauth"
)

func (purpose OAuthPurpose) Valid() bool {
	switch purpose {
	case OAuthPurposeLogin, OAuthPurposeLink, OAuthPurposeReauth:
		return true
	default:
		return false
	}
}

// Persisted reports whether today's database schema can represent the purpose.
func (purpose OAuthPurpose) Persisted() bool {
	return purpose == OAuthPurposeLogin || purpose == OAuthPurposeLink
}

// OAuthBinding is the purpose-bound authority carried between the start,
// callback, and finalize transitions. Zero UUIDs mean that binding is absent.
type OAuthBinding struct {
	PreSessionID uuid.UUID
	UserID       uuid.UUID
	SessionID    uuid.UUID
}

func (binding OAuthBinding) ValidForFlow(purpose OAuthPurpose) bool {
	switch purpose {
	case OAuthPurposeLogin:
		return binding.PreSessionID != uuid.Nil && binding.UserID == uuid.Nil &&
			binding.SessionID == uuid.Nil
	case OAuthPurposeLink, OAuthPurposeReauth:
		return binding.PreSessionID == uuid.Nil && binding.UserID == uuid.Nil &&
			binding.SessionID != uuid.Nil
	default:
		return false
	}
}

func (binding OAuthBinding) ValidForTicket(purpose OAuthPurpose) bool {
	switch purpose {
	case OAuthPurposeLogin:
		return binding.PreSessionID != uuid.Nil && binding.UserID == uuid.Nil &&
			binding.SessionID == uuid.Nil
	case OAuthPurposeLink, OAuthPurposeReauth:
		return binding.PreSessionID == uuid.Nil && binding.UserID != uuid.Nil &&
			binding.SessionID != uuid.Nil
	default:
		return false
	}
}

type OAuthFlowStatus string

const (
	OAuthFlowStatusPending  OAuthFlowStatus = "pending"
	OAuthFlowStatusConsumed OAuthFlowStatus = "consumed"
	OAuthFlowStatusExpired  OAuthFlowStatus = "expired"
)

// OAuthFlow contains only persistence-safe material. The raw state and PKCE
// verifier are deliberately absent and must remain inside the service layer.
type OAuthFlow struct {
	ID                     uuid.UUID
	Provider               string
	Purpose                OAuthPurpose
	Binding                OAuthBinding
	StateHash              []byte `json:"-"`
	RedirectURI            string `json:"-"`
	PKCEVerifierCiphertext []byte `json:"-"`
	KeyID                  string
	Status                 OAuthFlowStatus
	ExpiresAt              time.Time
	ConsumedAt             *time.Time
	CreatedAt              time.Time
}

type OAuthCompletionTicketStatus string

const (
	OAuthCompletionTicketStatusIssued   OAuthCompletionTicketStatus = "issued"
	OAuthCompletionTicketStatusConsumed OAuthCompletionTicketStatus = "consumed"
	OAuthCompletionTicketStatusExpired  OAuthCompletionTicketStatus = "expired"
)

// OAuthCompletionTicket is the protected callback receipt. Provider profile
// plaintext and the raw ticket never cross into the persistence adapter.
type OAuthCompletionTicket struct {
	ID                        uuid.UUID
	FlowID                    uuid.UUID
	TicketHash                []byte `json:"-"`
	Provider                  string
	ProviderSubject           string `json:"-"`
	Purpose                   OAuthPurpose
	Binding                   OAuthBinding
	RedirectURI               string `json:"-"`
	ProviderProfileCiphertext []byte `json:"-"`
	KeyID                     string
	Status                    OAuthCompletionTicketStatus
	ExpiresAt                 time.Time
	ConsumedAt                *time.Time
	CreatedAt                 time.Time
}
