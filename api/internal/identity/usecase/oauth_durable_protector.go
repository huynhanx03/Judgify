package usecase

import (
	"bytes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/oauth"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

const (
	oauthStatePrefix             = "jos1."
	oauthTicketPrefix            = "jot1."
	oauthCredentialEntropyBytes  = 32
	oauthMaximumLifetime         = 15 * time.Minute
	oauthMaximumCiphertextBytes  = 32 << 10
	oauthMaximumProfileJSONBytes = 16 << 10
	oauthStateHashDomain         = "judgify/identity/oauth-state-hmac/v1/"
	oauthTicketHashDomain        = "judgify/identity/oauth-ticket-hmac/v1/"
	oauthPKCEAEADDomain          = "judgify/identity/oauth-pkce-aead/v1/"
	oauthProfileAEADDomain       = "judgify/identity/oauth-profile-aead/v1/"
)

var (
	ErrInvalidOAuthDurableProtector = errors.New("identity: invalid oauth durable protector")
	ErrOAuthCredentialProtection    = errors.New("identity: oauth credential protection failed")
)

type OAuthDurableProtectorConfig struct {
	Secret []byte
	KeyID  string
	Random io.Reader
}

type OAuthFlowProtectionRequest struct {
	ID          uuid.UUID
	Provider    string
	Purpose     entity.OAuthPurpose
	Binding     entity.OAuthBinding
	RedirectURI string
	IssuedAt    time.Time
	ExpiresAt   time.Time
}

// ProtectedOAuthFlow is service-private transport material. State must be sent
// directly to the provider and must never be persisted or logged; the raw PKCE
// verifier is recoverable only through OpenPKCE after loading the protected row.
type ProtectedOAuthFlow struct {
	State         string `json:"-"`
	PKCEChallenge string `json:"-"`
	Flow          entity.OAuthFlow
}

type OAuthCompletionProtectionRequest struct {
	ID        uuid.UUID
	Flow      *entity.OAuthFlow
	Profile   oauth.Profile
	Binding   entity.OAuthBinding
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// ProtectedOAuthCompletion keeps the raw one-time ticket in the service layer
// while exposing a persistence-safe record to the repository port.
type ProtectedOAuthCompletion struct {
	Ticket string `json:"-"`
	Record entity.OAuthCompletionTicket
}

type OAuthDurableProtector struct {
	keyID         string
	stateHashKey  []byte
	ticketHashKey []byte
	pkceAEAD      cipher.AEAD
	profileAEAD   cipher.AEAD
	random        io.Reader
}

func NewOAuthDurableProtector(
	config OAuthDurableProtectorConfig,
) (*OAuthDurableProtector, error) {
	if config.Random == nil {
		config.Random = defaultCryptoRandom()
	}
	if !validOAuthKeyID(config.KeyID) || len(config.Secret) < derivedKeyBytes ||
		len(config.Secret) > 4096 || strings.Contains(string(config.Secret), "${") ||
		config.Random == nil {
		return nil, ErrInvalidOAuthDurableProtector
	}
	stateHashKey, err := deriveIdentityKey(config.Secret, oauthStateHashDomain+config.KeyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	ticketHashKey, err := deriveIdentityKey(config.Secret, oauthTicketHashDomain+config.KeyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	pkceKey, err := deriveIdentityKey(config.Secret, oauthPKCEAEADDomain+config.KeyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	profileKey, err := deriveIdentityKey(config.Secret, oauthProfileAEADDomain+config.KeyID)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	pkceAEAD, err := newIdentityAEAD(pkceKey)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	profileAEAD, err := newIdentityAEAD(profileKey)
	if err != nil {
		return nil, errors.Join(ErrInvalidOAuthDurableProtector, err)
	}
	return &OAuthDurableProtector{
		keyID: config.KeyID, stateHashKey: stateHashKey,
		ticketHashKey: ticketHashKey, pkceAEAD: pkceAEAD,
		profileAEAD: profileAEAD, random: config.Random,
	}, nil
}

func (protector *OAuthDurableProtector) ProtectFlow(
	request OAuthFlowProtectionRequest,
) (*ProtectedOAuthFlow, error) {
	issuedAt := oauthCanonicalTime(request.IssuedAt)
	expiresAt := oauthCanonicalTime(request.ExpiresAt)
	if protector == nil || protector.random == nil || request.ID == uuid.Nil ||
		!validOAuthProvider(request.Provider) || !request.Purpose.Valid() ||
		!request.Binding.ValidForFlow(request.Purpose) ||
		!validOAuthRedirectURI(request.RedirectURI) ||
		!validOAuthLifetime(issuedAt, expiresAt) {
		return nil, ErrOAuthCredentialProtection
	}
	state, err := protector.newCredential(oauthStatePrefix)
	if err != nil {
		return nil, err
	}
	verifierEntropy := make([]byte, oauthCredentialEntropyBytes)
	if _, err := io.ReadFull(protector.random, verifierEntropy); err != nil {
		return nil, errors.Join(ErrOAuthCredentialProtection, err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierEntropy)
	challengeDigest := sha256.Sum256([]byte(verifier))
	flow := entity.OAuthFlow{
		ID: request.ID, Provider: request.Provider, Purpose: request.Purpose,
		Binding: request.Binding, StateHash: protector.hash(protector.stateHashKey, state),
		RedirectURI: request.RedirectURI, KeyID: protector.keyID,
		Status: entity.OAuthFlowStatusPending, ExpiresAt: expiresAt,
		CreatedAt: issuedAt,
	}
	ciphertext, err := protector.seal(
		protector.pkceAEAD,
		[]byte(verifier),
		oauthFlowAssociatedData(flow),
	)
	if err != nil {
		return nil, err
	}
	flow.PKCEVerifierCiphertext = ciphertext
	return &ProtectedOAuthFlow{
		State:         state,
		PKCEChallenge: base64.RawURLEncoding.EncodeToString(challengeDigest[:]),
		Flow:          flow,
	}, nil
}

func (protector *OAuthDurableProtector) HashState(raw string) ([]byte, error) {
	if protector == nil || !validOAuthCredential(raw, oauthStatePrefix) {
		return nil, ErrOAuthCredentialProtection
	}
	return protector.hash(protector.stateHashKey, raw), nil
}

func (protector *OAuthDurableProtector) OpenPKCE(
	flow *entity.OAuthFlow,
	now time.Time,
) (string, error) {
	if protector == nil || !validProtectedOAuthFlow(flow) || flow.KeyID != protector.keyID ||
		now.IsZero() || !flow.ExpiresAt.After(now.UTC()) ||
		flow.Status != entity.OAuthFlowStatusPending {
		return "", ErrOAuthCredentialProtection
	}
	plaintext, err := protector.open(
		protector.pkceAEAD,
		flow.PKCEVerifierCiphertext,
		oauthFlowAssociatedData(*flow),
	)
	if err != nil {
		return "", err
	}
	verifier := string(plaintext)
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(verifier)
	if decodeErr != nil || len(decoded) != oauthCredentialEntropyBytes ||
		len(verifier) != base64.RawURLEncoding.EncodedLen(oauthCredentialEntropyBytes) {
		return "", ErrOAuthCredentialProtection
	}
	return verifier, nil
}

func (protector *OAuthDurableProtector) ProtectCompletion(
	request OAuthCompletionProtectionRequest,
) (*ProtectedOAuthCompletion, error) {
	issuedAt := oauthCanonicalTime(request.IssuedAt)
	expiresAt := oauthCanonicalTime(request.ExpiresAt)
	if protector == nil || request.Flow == nil || request.ID == uuid.Nil ||
		!validProtectedOAuthFlow(request.Flow) || request.Flow.KeyID != protector.keyID ||
		request.Flow.Status != entity.OAuthFlowStatusPending ||
		!request.Binding.ValidForTicket(request.Flow.Purpose) ||
		!oauthTicketBindingMatchesFlow(request.Binding, request.Flow.Binding, request.Flow.Purpose) ||
		!validOAuthProfile(request.Profile) ||
		!validOAuthLifetime(issuedAt, expiresAt) ||
		!request.Flow.ExpiresAt.After(issuedAt) {
		return nil, ErrOAuthCredentialProtection
	}
	rawTicket, err := protector.newCredential(oauthTicketPrefix)
	if err != nil {
		return nil, err
	}
	record := entity.OAuthCompletionTicket{
		ID: request.ID, FlowID: request.Flow.ID,
		TicketHash: protector.hash(protector.ticketHashKey, rawTicket),
		Provider:   request.Flow.Provider, ProviderSubject: request.Profile.Subject,
		Purpose: request.Flow.Purpose, Binding: request.Binding,
		RedirectURI: request.Flow.RedirectURI, KeyID: protector.keyID,
		Status:    entity.OAuthCompletionTicketStatusIssued,
		ExpiresAt: expiresAt, CreatedAt: issuedAt,
	}
	profileJSON, err := json.Marshal(request.Profile)
	if err != nil || len(profileJSON) == 0 || len(profileJSON) > oauthMaximumProfileJSONBytes {
		return nil, ErrOAuthCredentialProtection
	}
	ciphertext, err := protector.seal(
		protector.profileAEAD,
		profileJSON,
		oauthTicketAssociatedData(record),
	)
	if err != nil {
		return nil, err
	}
	record.ProviderProfileCiphertext = ciphertext
	return &ProtectedOAuthCompletion{Ticket: rawTicket, Record: record}, nil
}

func (protector *OAuthDurableProtector) HashTicket(raw string) ([]byte, error) {
	if protector == nil || !validOAuthCredential(raw, oauthTicketPrefix) {
		return nil, ErrOAuthCredentialProtection
	}
	return protector.hash(protector.ticketHashKey, raw), nil
}

func (protector *OAuthDurableProtector) OpenProfile(
	ticket *entity.OAuthCompletionTicket,
) (oauth.Profile, error) {
	if protector == nil || !validProtectedOAuthTicket(ticket) || ticket.KeyID != protector.keyID {
		return oauth.Profile{}, ErrOAuthCredentialProtection
	}
	plaintext, err := protector.open(
		protector.profileAEAD,
		ticket.ProviderProfileCiphertext,
		oauthTicketAssociatedData(*ticket),
	)
	if err != nil {
		return oauth.Profile{}, err
	}
	var profile oauth.Profile
	if len(plaintext) > oauthMaximumProfileJSONBytes ||
		json.Unmarshal(plaintext, &profile) != nil || !validOAuthProfile(profile) ||
		profile.Subject != ticket.ProviderSubject {
		return oauth.Profile{}, ErrOAuthCredentialProtection
	}
	return profile, nil
}

func (protector *OAuthDurableProtector) newCredential(prefix string) (string, error) {
	entropy := make([]byte, oauthCredentialEntropyBytes)
	if _, err := io.ReadFull(protector.random, entropy); err != nil {
		return "", errors.Join(ErrOAuthCredentialProtection, err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(entropy), nil
}

func (protector *OAuthDurableProtector) hash(key []byte, raw string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(raw))
	return mac.Sum(nil)
}

func (protector *OAuthDurableProtector) seal(
	aead cipher.AEAD,
	plaintext []byte,
	associatedData []byte,
) ([]byte, error) {
	if protector == nil || aead == nil || len(plaintext) == 0 ||
		len(plaintext) > oauthMaximumProfileJSONBytes || len(associatedData) == 0 {
		return nil, ErrOAuthCredentialProtection
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(protector.random, nonce); err != nil {
		return nil, errors.Join(ErrOAuthCredentialProtection, err)
	}
	sealed := make([]byte, 0, len(nonce)+len(plaintext)+aead.Overhead())
	sealed = append(sealed, nonce...)
	return aead.Seal(sealed, nonce, plaintext, associatedData), nil
}

func (protector *OAuthDurableProtector) open(
	aead cipher.AEAD,
	ciphertext []byte,
	associatedData []byte,
) ([]byte, error) {
	if protector == nil || aead == nil || len(associatedData) == 0 ||
		len(ciphertext) <= aead.NonceSize()+aead.Overhead() ||
		len(ciphertext) > oauthMaximumCiphertextBytes {
		return nil, ErrOAuthCredentialProtection
	}
	nonceSize := aead.NonceSize()
	plaintext, err := aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], associatedData)
	if err != nil {
		return nil, ErrOAuthCredentialProtection
	}
	return plaintext, nil
}

func oauthFlowAssociatedData(flow entity.OAuthFlow) []byte {
	return oauthAssociatedData(
		"flow-pkce", flow.ID.String(), flow.KeyID, flow.Provider, string(flow.Purpose),
		flow.Binding.PreSessionID.String(), flow.Binding.UserID.String(),
		flow.Binding.SessionID.String(), flow.RedirectURI,
		flow.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
}

func oauthTicketAssociatedData(ticket entity.OAuthCompletionTicket) []byte {
	return oauthAssociatedData(
		"completion-profile", ticket.ID.String(), ticket.FlowID.String(), ticket.KeyID,
		ticket.Provider, ticket.ProviderSubject, string(ticket.Purpose),
		ticket.Binding.PreSessionID.String(), ticket.Binding.UserID.String(),
		ticket.Binding.SessionID.String(), ticket.RedirectURI,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
}

func oauthAssociatedData(fields ...string) []byte {
	var buffer bytes.Buffer
	for _, field := range fields {
		_ = binary.Write(&buffer, binary.BigEndian, uint32(len(field)))
		_, _ = buffer.WriteString(field)
	}
	return buffer.Bytes()
}

func validProtectedOAuthFlow(flow *entity.OAuthFlow) bool {
	return flow != nil && flow.ID != uuid.Nil && validOAuthProvider(flow.Provider) &&
		flow.Purpose.Valid() && flow.Binding.ValidForFlow(flow.Purpose) &&
		len(flow.StateHash) == sha256.Size && validOAuthRedirectURI(flow.RedirectURI) &&
		len(flow.PKCEVerifierCiphertext) > 1 &&
		len(flow.PKCEVerifierCiphertext) <= oauthMaximumCiphertextBytes &&
		validOAuthKeyID(flow.KeyID) && !flow.ExpiresAt.IsZero() && !flow.CreatedAt.IsZero() &&
		flow.ExpiresAt.After(flow.CreatedAt)
}

func validProtectedOAuthTicket(ticket *entity.OAuthCompletionTicket) bool {
	return ticket != nil && ticket.ID != uuid.Nil && ticket.FlowID != uuid.Nil &&
		len(ticket.TicketHash) == sha256.Size && validOAuthProvider(ticket.Provider) &&
		validOAuthBoundedText(ticket.ProviderSubject, 256, false) && ticket.Purpose.Valid() &&
		ticket.Binding.ValidForTicket(ticket.Purpose) &&
		validOAuthRedirectURI(ticket.RedirectURI) &&
		len(ticket.ProviderProfileCiphertext) > 1 &&
		len(ticket.ProviderProfileCiphertext) <= oauthMaximumCiphertextBytes &&
		validOAuthKeyID(ticket.KeyID) && !ticket.ExpiresAt.IsZero() &&
		!ticket.CreatedAt.IsZero() && ticket.ExpiresAt.After(ticket.CreatedAt) &&
		validOAuthTicketTransition(ticket)
}

func validOAuthTicketTransition(ticket *entity.OAuthCompletionTicket) bool {
	switch ticket.Status {
	case entity.OAuthCompletionTicketStatusIssued:
		return ticket.ConsumedAt == nil
	case entity.OAuthCompletionTicketStatusConsumed:
		return ticket.ConsumedAt != nil && !ticket.ConsumedAt.Before(ticket.CreatedAt) &&
			ticket.ExpiresAt.After(*ticket.ConsumedAt)
	default:
		return false
	}
}

func validOAuthLifetime(issuedAt, expiresAt time.Time) bool {
	return !issuedAt.IsZero() && !expiresAt.IsZero() && expiresAt.After(issuedAt) &&
		expiresAt.Sub(issuedAt) <= oauthMaximumLifetime
}

// PostgreSQL timestamps round-trip at microsecond precision. Canonicalizing
// before sealing keeps expiry-bound associated data stable after persistence.
func oauthCanonicalTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func validOAuthCredential(raw, prefix string) bool {
	expectedLength := len(prefix) + base64.RawURLEncoding.EncodedLen(oauthCredentialEntropyBytes)
	if len(raw) != expectedLength || !strings.HasPrefix(raw, prefix) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, prefix))
	return err == nil && len(decoded) == oauthCredentialEntropyBytes
}

func validOAuthKeyID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' ||
			character == '.' {
			continue
		}
		return false
	}
	return true
}

func validOAuthProvider(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validOAuthRedirectURI(value string) bool {
	if !validOAuthBoundedText(value, 2048, false) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Fragment == ""
}

func validOAuthProfile(profile oauth.Profile) bool {
	return validOAuthBoundedText(profile.Subject, 256, false) &&
		validOAuthBoundedText(profile.Email, 320, true) &&
		validOAuthBoundedText(profile.DisplayName, 512, true) &&
		validOAuthBoundedText(profile.Login, 256, true) &&
		validOAuthBoundedText(profile.AvatarURL, 2048, true)
}

func validOAuthBoundedText(value string, maximumBytes int, allowEmpty bool) bool {
	if (!allowEmpty && value == "") || len(value) > maximumBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func oauthTicketBindingMatchesFlow(
	ticket entity.OAuthBinding,
	flow entity.OAuthBinding,
	purpose entity.OAuthPurpose,
) bool {
	switch purpose {
	case entity.OAuthPurposeLogin:
		return ticket.PreSessionID == flow.PreSessionID
	case entity.OAuthPurposeLink, entity.OAuthPurposeReauth:
		return ticket.SessionID == flow.SessionID
	default:
		return false
	}
}
