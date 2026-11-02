package usecase

import (
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net/mail"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

const (
	defaultMaximumRecoveryEmailBytes = 320
	maximumRecoveryContactKeyCount   = 8
	maximumRecoveryContactCiphertext = 1024
	recoveryContactKindEmail         = "email"
	recoveryContactAEADDomain        = "judgify/identity/recovery-contact-aead/v2/"
	recoveryContactLookupDomain      = "judgify/identity/recovery-contact-lookup/v2/"
	recoveryContactFingerprintDomain = "judgify/identity/recovery-contact-fingerprint/v2/"
	recoveryVerificationTokenDomain  = "judgify/identity/recovery-verification-token-hmac/v1/"
	recoveryInvitationTokenDomain    = "judgify/identity/account-invitation-token-hmac/v1/"
	recoveryPasswordResetTokenDomain = "judgify/identity/password-reset-token-hmac/v2/"
	recoveryRateLimitDomain          = "judgify/identity/recovery-rate-limit-hmac/v1/"
	recoveryVerificationTokenPrefix  = "jcv1."
	recoveryInvitationTokenPrefix    = "jai1."
)

var (
	ErrInvalidRecoveryContactProtector = errors.New("identity: invalid recovery contact protector")
	ErrInvalidRecoveryEmail            = errors.New("identity: invalid recovery email")
	ErrRecoveryContactAuthentication   = errors.New("identity: recovery contact authentication failed")
)

type RecoveryContactProtectorConfig struct {
	ActiveKeyID       string
	Keys              map[string][]byte
	MaximumEmailBytes int
	Random            io.Reader
}

type RecoveryContactMetadata struct {
	UserID    uuid.UUID
	ContactID uuid.UUID
	Kind      string
	Version   uint64
}

type recoveryContactKeyMaterial struct {
	aead                  cipher.AEAD
	lookupKey             []byte
	fingerprintKey        []byte
	verificationTokenKey  []byte
	invitationTokenKey    []byte
	passwordResetTokenKey []byte
	rateLimitKey          []byte
}

type recoveryOneTimeTokenKind uint8

const (
	recoveryOneTimeTokenVerification recoveryOneTimeTokenKind = iota + 1
	recoveryOneTimeTokenInvitation
	recoveryOneTimeTokenPasswordReset
)

// RecoveryContactProtector is an application-owned keyring for encrypted
// recovery destinations and their domain-separated blind indexes. It never
// retains address plaintext.
type RecoveryContactProtector struct {
	activeKeyID       string
	keys              map[string]recoveryContactKeyMaterial
	keyIDs            []string
	maximumEmailBytes int
	random            io.Reader
}

func NewRecoveryContactProtector(
	config RecoveryContactProtectorConfig,
) (*RecoveryContactProtector, error) {
	if config.MaximumEmailBytes == 0 {
		config.MaximumEmailBytes = defaultMaximumRecoveryEmailBytes
	}
	if config.Random == nil {
		config.Random = defaultCryptoRandom()
	}
	if !validSecurityEmailKeyID(config.ActiveKeyID) || len(config.Keys) == 0 ||
		len(config.Keys) > maximumRecoveryContactKeyCount ||
		config.MaximumEmailBytes < 64 || config.MaximumEmailBytes > defaultMaximumRecoveryEmailBytes ||
		config.Random == nil {
		return nil, ErrInvalidRecoveryContactProtector
	}

	keys := make(map[string]recoveryContactKeyMaterial, len(config.Keys))
	for keyID, secret := range config.Keys {
		if !validSecurityEmailKeyID(keyID) || len(secret) < minimumSecretBytes ||
			len(secret) > 4096 || strings.Contains(string(secret), "${") {
			return nil, ErrInvalidRecoveryContactProtector
		}
		aeadKey, err := deriveIdentityKey(secret, recoveryContactAEADDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		aead, err := newIdentityAEAD(aeadKey)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		lookupKey, err := deriveIdentityKey(secret, recoveryContactLookupDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		fingerprintKey, err := deriveIdentityKey(secret, recoveryContactFingerprintDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		verificationTokenKey, err := deriveIdentityKey(secret, recoveryVerificationTokenDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		invitationTokenKey, err := deriveIdentityKey(secret, recoveryInvitationTokenDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		passwordResetTokenKey, err := deriveIdentityKey(secret, recoveryPasswordResetTokenDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		rateLimitKey, err := deriveIdentityKey(secret, recoveryRateLimitDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidRecoveryContactProtector, err)
		}
		keys[keyID] = recoveryContactKeyMaterial{
			aead: aead, lookupKey: lookupKey, fingerprintKey: fingerprintKey,
			verificationTokenKey:  verificationTokenKey,
			invitationTokenKey:    invitationTokenKey,
			passwordResetTokenKey: passwordResetTokenKey,
			rateLimitKey:          rateLimitKey,
		}
	}
	if _, exists := keys[config.ActiveKeyID]; !exists {
		return nil, ErrInvalidRecoveryContactProtector
	}

	keyIDs := make([]string, 0, len(keys))
	for keyID := range keys {
		if keyID != config.ActiveKeyID {
			keyIDs = append(keyIDs, keyID)
		}
	}
	sort.Strings(keyIDs)
	keyIDs = append([]string{config.ActiveKeyID}, keyIDs...)

	return &RecoveryContactProtector{
		activeKeyID: config.ActiveKeyID, keys: keys,
		keyIDs:            keyIDs,
		maximumEmailBytes: config.MaximumEmailBytes,
		random:            config.Random,
	}, nil
}

func (protector *RecoveryContactProtector) RateLimitHash(subject string) ([]byte, error) {
	if protector == nil || len(subject) == 0 || len(subject) > 4096 || !utf8.ValidString(subject) {
		return nil, ErrInvalidRecoveryContactProtector
	}
	material, exists := protector.keys[protector.activeKeyID]
	if !exists || len(material.rateLimitKey) == 0 {
		return nil, ErrInvalidRecoveryContactProtector
	}
	return recoveryContactHash(material.rateLimitKey, subject), nil
}

func (protector *RecoveryContactProtector) NewVerificationToken() (
	string,
	entity.KeyedTokenHash,
	error,
) {
	return protector.newOneTimeToken(
		recoveryVerificationTokenPrefix,
		recoveryOneTimeTokenVerification,
	)
}

func (protector *RecoveryContactProtector) NewInvitationToken() (
	string,
	entity.KeyedTokenHash,
	error,
) {
	return protector.newOneTimeToken(
		recoveryInvitationTokenPrefix,
		recoveryOneTimeTokenInvitation,
	)
}

func (protector *RecoveryContactProtector) NewPasswordResetToken() (
	string,
	entity.KeyedTokenHash,
	error,
) {
	return protector.newOneTimeToken(
		passwordResetTokenPrefix,
		recoveryOneTimeTokenPasswordReset,
	)
}

func (protector *RecoveryContactProtector) VerificationTokenCandidates(
	raw string,
	maximumBytes int,
) ([]entity.KeyedTokenHash, error) {
	return protector.oneTimeTokenCandidates(
		raw,
		recoveryVerificationTokenPrefix,
		maximumBytes,
		recoveryOneTimeTokenVerification,
	)
}

func (protector *RecoveryContactProtector) InvitationTokenCandidates(
	raw string,
	maximumBytes int,
) ([]entity.KeyedTokenHash, error) {
	return protector.oneTimeTokenCandidates(
		raw,
		recoveryInvitationTokenPrefix,
		maximumBytes,
		recoveryOneTimeTokenInvitation,
	)
}

func (protector *RecoveryContactProtector) PasswordResetTokenCandidates(
	raw string,
	maximumBytes int,
) ([]entity.KeyedTokenHash, error) {
	return protector.oneTimeTokenCandidates(
		raw,
		passwordResetTokenPrefix,
		maximumBytes,
		recoveryOneTimeTokenPasswordReset,
	)
}

func (protector *RecoveryContactProtector) newOneTimeToken(
	prefix string,
	kind recoveryOneTimeTokenKind,
) (string, entity.KeyedTokenHash, error) {
	if protector == nil || protector.random == nil || len(protector.keyIDs) == 0 {
		return "", entity.KeyedTokenHash{}, ErrInvalidRecoveryContactProtector
	}
	entropy := make([]byte, passwordResetEntropyBytes)
	if _, err := io.ReadFull(protector.random, entropy); err != nil {
		return "", entity.KeyedTokenHash{}, errors.Join(ErrInvalidRecoveryContactProtector, err)
	}
	raw := prefix + base64.RawURLEncoding.EncodeToString(entropy)
	material := protector.keys[protector.activeKeyID]
	key, ok := recoveryOneTimeTokenKey(material, kind)
	if !ok {
		return "", entity.KeyedTokenHash{}, ErrInvalidRecoveryContactProtector
	}
	return raw, entity.KeyedTokenHash{
		KeyID: protector.activeKeyID,
		Hash:  recoveryContactHash(key, raw),
	}, nil
}

func (protector *RecoveryContactProtector) oneTimeTokenCandidates(
	raw string,
	prefix string,
	maximumBytes int,
	kind recoveryOneTimeTokenKind,
) ([]entity.KeyedTokenHash, error) {
	expectedLength := len(prefix) + base64.RawURLEncoding.EncodedLen(passwordResetEntropyBytes)
	if protector == nil || maximumBytes < expectedLength || len(raw) > maximumBytes ||
		len(raw) != expectedLength || !strings.HasPrefix(raw, prefix) {
		return nil, ErrRecoveryContactAuthentication
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, prefix))
	if err != nil || len(decoded) != passwordResetEntropyBytes {
		return nil, ErrRecoveryContactAuthentication
	}

	candidates := make([]entity.KeyedTokenHash, 0, len(protector.keyIDs))
	for _, keyID := range protector.keyIDs {
		material := protector.keys[keyID]
		key, ok := recoveryOneTimeTokenKey(material, kind)
		if !ok {
			return nil, ErrInvalidRecoveryContactProtector
		}
		candidates = append(candidates, entity.KeyedTokenHash{
			KeyID: keyID,
			Hash:  recoveryContactHash(key, raw),
		})
	}
	return candidates, nil
}

func recoveryOneTimeTokenKey(
	material recoveryContactKeyMaterial,
	kind recoveryOneTimeTokenKind,
) ([]byte, bool) {
	var key []byte
	switch kind {
	case recoveryOneTimeTokenVerification:
		key = material.verificationTokenKey
	case recoveryOneTimeTokenInvitation:
		key = material.invitationTokenKey
	case recoveryOneTimeTokenPasswordReset:
		key = material.passwordResetTokenKey
	default:
		return nil, false
	}
	return key, len(key) == sha256.Size
}

// NormalizeRecoveryEmail returns the canonical account-recovery address. The
// launch deliberately accepts bounded ASCII mailbox syntax only; display names,
// comments and internationalized mailbox forms are rejected rather than
// normalized ambiguously.
func NormalizeRecoveryEmail(value string, maximumBytes int) (string, error) {
	if maximumBytes == 0 {
		maximumBytes = defaultMaximumRecoveryEmailBytes
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maximumBytes || !utf8.ValidString(value) ||
		strings.Count(value, "@") != 1 || strings.ContainsAny(value, "\r\n\t ") {
		return "", ErrInvalidRecoveryEmail
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return "", ErrInvalidRecoveryEmail
		}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || parsed.Address != value {
		return "", ErrInvalidRecoveryEmail
	}
	separator := strings.LastIndexByte(value, '@')
	local, domain := value[:separator], value[separator+1:]
	if local == "" || domain == "" || len(local) > 64 || len(domain) > 253 ||
		strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") ||
		strings.Contains(local, "..") || strings.HasPrefix(domain, ".") ||
		strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", ErrInvalidRecoveryEmail
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") ||
			strings.HasSuffix(label, "-") {
			return "", ErrInvalidRecoveryEmail
		}
	}
	return strings.ToLower(value), nil
}

func MaskRecoveryEmail(value string) (string, error) {
	normalized, err := NormalizeRecoveryEmail(value, defaultMaximumRecoveryEmailBytes)
	if err != nil {
		return "", err
	}
	separator := strings.LastIndexByte(normalized, '@')
	return normalized[:1] + "***" + normalized[separator:], nil
}

func (protector *RecoveryContactProtector) Protect(
	metadata RecoveryContactMetadata,
	email string,
) ([]byte, string, error) {
	if protector == nil || !validRecoveryContactMetadata(metadata) {
		return nil, "", ErrInvalidRecoveryContactProtector
	}
	normalized, err := NormalizeRecoveryEmail(email, protector.maximumEmailBytes)
	if err != nil {
		return nil, "", err
	}
	material := protector.keys[protector.activeKeyID]
	nonce := make([]byte, material.aead.NonceSize())
	if _, err := io.ReadFull(protector.random, nonce); err != nil {
		return nil, "", errors.Join(ErrInvalidRecoveryContactProtector, err)
	}
	result := make([]byte, 0, len(nonce)+len(normalized)+material.aead.Overhead())
	result = append(result, nonce...)
	result = material.aead.Seal(
		result,
		nonce,
		[]byte(normalized),
		recoveryContactAAD(metadata, protector.activeKeyID),
	)
	return result, protector.activeKeyID, nil
}

func (protector *RecoveryContactProtector) Open(
	metadata RecoveryContactMetadata,
	keyID string,
	ciphertext []byte,
) (string, error) {
	if protector == nil || !validRecoveryContactMetadata(metadata) ||
		!validSecurityEmailKeyID(keyID) || len(ciphertext) == 0 ||
		len(ciphertext) > maximumRecoveryContactCiphertext {
		return "", ErrInvalidRecoveryContactProtector
	}
	material, exists := protector.keys[keyID]
	if !exists || len(ciphertext) <= material.aead.NonceSize()+material.aead.Overhead() {
		return "", ErrRecoveryContactAuthentication
	}
	nonceSize := material.aead.NonceSize()
	plaintext, err := material.aead.Open(
		nil,
		ciphertext[:nonceSize],
		ciphertext[nonceSize:],
		recoveryContactAAD(metadata, keyID),
	)
	if err != nil {
		return "", ErrRecoveryContactAuthentication
	}
	normalized, err := NormalizeRecoveryEmail(string(plaintext), protector.maximumEmailBytes)
	if err != nil || !hmac.Equal(plaintext, []byte(normalized)) {
		return "", ErrRecoveryContactAuthentication
	}
	return normalized, nil
}

func (protector *RecoveryContactProtector) ActiveHashes(
	email string,
) (lookupHash, fingerprintHash []byte, keyID string, err error) {
	if protector == nil {
		return nil, nil, "", ErrInvalidRecoveryContactProtector
	}
	normalized, err := NormalizeRecoveryEmail(email, protector.maximumEmailBytes)
	if err != nil {
		return nil, nil, "", err
	}
	material := protector.keys[protector.activeKeyID]
	return recoveryContactHash(material.lookupKey, normalized),
		recoveryContactHash(material.fingerprintKey, normalized),
		protector.activeKeyID, nil
}

func (protector *RecoveryContactProtector) LookupHash(
	keyID string,
	email string,
) ([]byte, error) {
	if protector == nil || !validSecurityEmailKeyID(keyID) {
		return nil, ErrInvalidRecoveryContactProtector
	}
	material, exists := protector.keys[keyID]
	if !exists {
		return nil, ErrInvalidRecoveryContactProtector
	}
	normalized, err := NormalizeRecoveryEmail(email, protector.maximumEmailBytes)
	if err != nil {
		return nil, err
	}
	return recoveryContactHash(material.lookupKey, normalized), nil
}

func recoveryContactHash(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func validRecoveryContactMetadata(metadata RecoveryContactMetadata) bool {
	return metadata.UserID != uuid.Nil && metadata.ContactID != uuid.Nil &&
		metadata.Kind == recoveryContactKindEmail && metadata.Version > 0 &&
		metadata.Version <= math.MaxInt64
}

func recoveryContactAAD(metadata RecoveryContactMetadata, keyID string) []byte {
	parts := [][]byte{
		[]byte("judgify/recovery-contact/v2"),
		metadata.UserID[:],
		metadata.ContactID[:],
		[]byte(metadata.Kind),
		[]byte(keyID),
	}
	result := make([]byte, 8)
	binary.BigEndian.PutUint64(result, metadata.Version)
	var length [4]byte
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		result = append(result, length[:]...)
		result = append(result, part...)
	}
	return result
}
