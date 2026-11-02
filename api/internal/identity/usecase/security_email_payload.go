package usecase

import (
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
)

const (
	securityEmailPayloadVersion          = 2
	defaultSecurityEmailCiphertextBytes  = 4096
	minimumSecurityEmailCiphertextBytes  = 512
	maximumSecurityEmailCiphertextBytes  = 64 << 10
	maximumSecurityEmailKeyCount         = 8
	securityEmailPayloadDerivationDomain = "judgify/identity/security-email-payload-aead/v2/"
)

var (
	ErrInvalidSecurityEmailPayloadCodec   = errors.New("identity: invalid security email payload codec")
	ErrSecurityEmailPayloadInvalid        = errors.New("identity: invalid security email payload")
	ErrSecurityEmailPayloadExpired        = errors.New("identity: security email payload expired")
	ErrSecurityEmailPayloadUnsupported    = errors.New("identity: unsupported security email payload")
	ErrSecurityEmailPayloadAuthentication = errors.New("identity: security email payload authentication failed")
)

type SecurityEmailPayloadCodecConfig struct {
	ActiveKeyID            string
	Keys                   map[string][]byte
	MaximumCiphertextBytes int
	Clock                  func() time.Time
	Random                 io.Reader
}

type SecurityEmailPayloadMetadata struct {
	DeliveryID             uuid.UUID
	CredentialID           uuid.UUID
	Kind                   ports.SecurityEmailKind
	ProviderIdempotencyKey string
	ExpiresAt              time.Time
}

type SecurityEmailPayloadCodec struct {
	activeKeyID            string
	aeadByKeyID            map[string]cipher.AEAD
	maximumCiphertextBytes int
	clock                  func() time.Time
	random                 io.Reader
}

type securityEmailPayloadWire struct {
	Version   uint8  `json:"v"`
	Recipient string `json:"recipient"`
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at_unix_nano"`
}

func NewSecurityEmailPayloadCodec(
	config SecurityEmailPayloadCodecConfig,
) (*SecurityEmailPayloadCodec, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Random == nil {
		config.Random = defaultCryptoRandom()
	}
	if config.MaximumCiphertextBytes == 0 {
		config.MaximumCiphertextBytes = defaultSecurityEmailCiphertextBytes
	}
	if !validSecurityEmailKeyID(config.ActiveKeyID) || len(config.Keys) == 0 ||
		len(config.Keys) > maximumSecurityEmailKeyCount ||
		config.MaximumCiphertextBytes < minimumSecurityEmailCiphertextBytes ||
		config.MaximumCiphertextBytes > maximumSecurityEmailCiphertextBytes ||
		config.Clock == nil || config.Random == nil {
		return nil, ErrInvalidSecurityEmailPayloadCodec
	}
	aeadByKeyID := make(map[string]cipher.AEAD, len(config.Keys))
	for keyID, secret := range config.Keys {
		if !validSecurityEmailKeyID(keyID) || len(secret) < minimumSecretBytes ||
			len(secret) > 4096 || strings.Contains(string(secret), "${") {
			return nil, ErrInvalidSecurityEmailPayloadCodec
		}
		key, err := deriveIdentityKey(secret, securityEmailPayloadDerivationDomain+keyID)
		if err != nil {
			return nil, errors.Join(ErrInvalidSecurityEmailPayloadCodec, err)
		}
		value, err := newIdentityAEAD(key)
		if err != nil {
			return nil, errors.Join(ErrInvalidSecurityEmailPayloadCodec, err)
		}
		aeadByKeyID[keyID] = value
	}
	if _, exists := aeadByKeyID[config.ActiveKeyID]; !exists {
		return nil, ErrInvalidSecurityEmailPayloadCodec
	}
	return &SecurityEmailPayloadCodec{
		activeKeyID: config.ActiveKeyID, aeadByKeyID: aeadByKeyID,
		maximumCiphertextBytes: config.MaximumCiphertextBytes,
		clock:                  config.Clock, random: config.Random,
	}, nil
}

func (codec *SecurityEmailPayloadCodec) SealPasswordReset(
	metadata SecurityEmailPayloadMetadata,
	message ports.PasswordResetDeliveryMessage,
) ([]byte, string, error) {
	if metadata.Kind != ports.SecurityEmailKindPasswordReset ||
		!validPasswordResetDeliveryMessage(message) || !message.ExpiresAt.Equal(metadata.ExpiresAt) ||
		message.ProviderIdempotencyKey != metadata.ProviderIdempotencyKey {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	return codec.seal(metadata, message.Recipient, message.Token, message.ExpiresAt)
}

func (codec *SecurityEmailPayloadCodec) SealRecoveryContactVerification(
	metadata SecurityEmailPayloadMetadata,
	message ports.RecoveryContactVerificationDeliveryMessage,
) ([]byte, string, error) {
	if metadata.Kind != ports.SecurityEmailKindRecoveryContactVerification ||
		!validRecoveryContactVerificationDeliveryMessage(message) ||
		!message.ExpiresAt.Equal(metadata.ExpiresAt) ||
		message.ProviderIdempotencyKey != metadata.ProviderIdempotencyKey {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	return codec.seal(metadata, message.Recipient, message.Token, message.ExpiresAt)
}

func (codec *SecurityEmailPayloadCodec) SealAccountInvitation(
	metadata SecurityEmailPayloadMetadata,
	message ports.AccountInvitationDeliveryMessage,
) ([]byte, string, error) {
	if metadata.Kind != ports.SecurityEmailKindAccountInvitation ||
		!validAccountInvitationDeliveryMessage(message) ||
		!message.ExpiresAt.Equal(metadata.ExpiresAt) ||
		message.ProviderIdempotencyKey != metadata.ProviderIdempotencyKey {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	return codec.seal(metadata, message.Recipient, message.Token, message.ExpiresAt)
}

func (codec *SecurityEmailPayloadCodec) seal(
	metadata SecurityEmailPayloadMetadata,
	recipient string,
	token string,
	expiresAt time.Time,
) ([]byte, string, error) {
	if codec == nil || !validSecurityEmailPayloadMetadata(metadata) ||
		!metadata.ExpiresAt.After(codec.clock().UTC()) || !expiresAt.Equal(metadata.ExpiresAt) {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	payload, err := json.Marshal(securityEmailPayloadWire{
		Version: securityEmailPayloadVersion, Recipient: recipient,
		Token: token, ExpiresAt: expiresAt.UTC().UnixNano(),
	})
	if err != nil {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	aead := codec.aeadByKeyID[codec.activeKeyID]
	maximumPlaintext := codec.maximumCiphertextBytes - aead.NonceSize() - aead.Overhead()
	if len(payload) == 0 || len(payload) > maximumPlaintext {
		return nil, "", ErrSecurityEmailPayloadInvalid
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(codec.random, nonce); err != nil {
		return nil, "", errors.Join(ErrSecurityEmailPayloadInvalid, err)
	}
	result := make([]byte, 0, len(nonce)+len(payload)+aead.Overhead())
	result = append(result, nonce...)
	result = aead.Seal(
		result,
		nonce,
		payload,
		securityEmailPayloadAAD(metadata, codec.activeKeyID),
	)
	return result, codec.activeKeyID, nil
}

func (codec *SecurityEmailPayloadCodec) OpenPasswordReset(
	metadata SecurityEmailPayloadMetadata,
	keyID string,
	ciphertext []byte,
	now time.Time,
) (ports.PasswordResetDeliveryMessage, error) {
	if metadata.Kind != ports.SecurityEmailKindPasswordReset {
		return ports.PasswordResetDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	wire, err := codec.open(metadata, keyID, ciphertext, now)
	if err != nil {
		return ports.PasswordResetDeliveryMessage{}, err
	}
	message := ports.PasswordResetDeliveryMessage{
		Recipient: wire.Recipient, Token: wire.Token,
		ExpiresAt:              time.Unix(0, wire.ExpiresAt).UTC(),
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	if !validPasswordResetDeliveryMessage(message) || !message.ExpiresAt.Equal(metadata.ExpiresAt) {
		return ports.PasswordResetDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	return message, nil
}

func (codec *SecurityEmailPayloadCodec) OpenRecoveryContactVerification(
	metadata SecurityEmailPayloadMetadata,
	keyID string,
	ciphertext []byte,
	now time.Time,
) (ports.RecoveryContactVerificationDeliveryMessage, error) {
	if metadata.Kind != ports.SecurityEmailKindRecoveryContactVerification {
		return ports.RecoveryContactVerificationDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	wire, err := codec.open(metadata, keyID, ciphertext, now)
	if err != nil {
		return ports.RecoveryContactVerificationDeliveryMessage{}, err
	}
	message := ports.RecoveryContactVerificationDeliveryMessage{
		Recipient: wire.Recipient, Token: wire.Token,
		ExpiresAt:              time.Unix(0, wire.ExpiresAt).UTC(),
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	if !validRecoveryContactVerificationDeliveryMessage(message) ||
		!message.ExpiresAt.Equal(metadata.ExpiresAt) {
		return ports.RecoveryContactVerificationDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	return message, nil
}

func (codec *SecurityEmailPayloadCodec) OpenAccountInvitation(
	metadata SecurityEmailPayloadMetadata,
	keyID string,
	ciphertext []byte,
	now time.Time,
) (ports.AccountInvitationDeliveryMessage, error) {
	if metadata.Kind != ports.SecurityEmailKindAccountInvitation {
		return ports.AccountInvitationDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	wire, err := codec.open(metadata, keyID, ciphertext, now)
	if err != nil {
		return ports.AccountInvitationDeliveryMessage{}, err
	}
	message := ports.AccountInvitationDeliveryMessage{
		Recipient: wire.Recipient, Token: wire.Token,
		ExpiresAt:              time.Unix(0, wire.ExpiresAt).UTC(),
		ProviderIdempotencyKey: metadata.ProviderIdempotencyKey,
	}
	if !validAccountInvitationDeliveryMessage(message) ||
		!message.ExpiresAt.Equal(metadata.ExpiresAt) {
		return ports.AccountInvitationDeliveryMessage{}, ErrSecurityEmailPayloadInvalid
	}
	return message, nil
}

func (codec *SecurityEmailPayloadCodec) open(
	metadata SecurityEmailPayloadMetadata,
	keyID string,
	ciphertext []byte,
	now time.Time,
) (securityEmailPayloadWire, error) {
	if codec == nil || !validSecurityEmailPayloadMetadata(metadata) || now.IsZero() ||
		len(ciphertext) == 0 || len(ciphertext) > codec.maximumCiphertextBytes {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadInvalid
	}
	if !metadata.ExpiresAt.After(now.UTC()) {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadExpired
	}
	aead, exists := codec.aeadByKeyID[keyID]
	if !exists || !validSecurityEmailKeyID(keyID) {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadUnsupported
	}
	if len(ciphertext) <= aead.NonceSize()+aead.Overhead() {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadInvalid
	}
	nonceSize := aead.NonceSize()
	plaintext, err := aead.Open(
		nil,
		ciphertext[:nonceSize],
		ciphertext[nonceSize:],
		securityEmailPayloadAAD(metadata, keyID),
	)
	if err != nil {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadAuthentication
	}
	var wire securityEmailPayloadWire
	if err := json.Unmarshal(plaintext, &wire); err != nil {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadInvalid
	}
	if wire.Version != securityEmailPayloadVersion {
		return securityEmailPayloadWire{}, ErrSecurityEmailPayloadUnsupported
	}
	return wire, nil
}

func securityEmailPayloadAAD(metadata SecurityEmailPayloadMetadata, keyID string) []byte {
	parts := [][]byte{
		[]byte("judgify/security-email-payload/v2"),
		metadata.DeliveryID[:],
		metadata.CredentialID[:],
		[]byte(metadata.Kind),
		[]byte(metadata.ProviderIdempotencyKey),
		[]byte(keyID),
	}
	total := 8
	for _, part := range parts {
		total += 4 + len(part)
	}
	result := make([]byte, 0, total)
	var expiry [8]byte
	binary.BigEndian.PutUint64(expiry[:], uint64(metadata.ExpiresAt.UTC().UnixNano()))
	result = append(result, expiry[:]...)
	var length [4]byte
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		result = append(result, length[:]...)
		result = append(result, part...)
	}
	return result
}

func validSecurityEmailPayloadMetadata(metadata SecurityEmailPayloadMetadata) bool {
	return metadata.DeliveryID != uuid.Nil && metadata.CredentialID != uuid.Nil &&
		(metadata.Kind == ports.SecurityEmailKindPasswordReset ||
			metadata.Kind == ports.SecurityEmailKindRecoveryContactVerification ||
			metadata.Kind == ports.SecurityEmailKindAccountInvitation) &&
		validProviderIdempotencyKey(metadata.ProviderIdempotencyKey) &&
		!metadata.ExpiresAt.IsZero()
}

func validPasswordResetDeliveryMessage(message ports.PasswordResetDeliveryMessage) bool {
	return validSecurityEmailRecipient(message.Recipient) &&
		validOneTimeEmailToken(message.Token, passwordResetTokenPrefix) &&
		!message.ExpiresAt.IsZero() && validProviderIdempotencyKey(message.ProviderIdempotencyKey)
}

func validRecoveryContactVerificationDeliveryMessage(
	message ports.RecoveryContactVerificationDeliveryMessage,
) bool {
	return validSecurityEmailRecipient(message.Recipient) &&
		validOneTimeEmailToken(message.Token, recoveryVerificationTokenPrefix) &&
		!message.ExpiresAt.IsZero() && validProviderIdempotencyKey(message.ProviderIdempotencyKey)
}

func validAccountInvitationDeliveryMessage(
	message ports.AccountInvitationDeliveryMessage,
) bool {
	return validSecurityEmailRecipient(message.Recipient) &&
		validOneTimeEmailToken(message.Token, recoveryInvitationTokenPrefix) &&
		!message.ExpiresAt.IsZero() && validProviderIdempotencyKey(message.ProviderIdempotencyKey)
}

func validSecurityEmailRecipient(value string) bool {
	return value != "" && len(value) <= 320 && utf8.ValidString(value) &&
		!strings.ContainsAny(value, "\r\n")
}

func validOneTimeEmailToken(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) ||
		len(value) != len(prefix)+base64.RawURLEncoding.EncodedLen(passwordResetEntropyBytes) ||
		!utf8.ValidString(value) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil && len(decoded) == passwordResetEntropyBytes
}

func validSecurityEmailKeyID(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value &&
		validBoundedText(value, 128)
}

func validProviderIdempotencyKey(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character <= 0x20 || character >= 0x7f {
			return false
		}
	}
	return true
}

func securityEmailProviderIdempotencyKey(deliveryID uuid.UUID) string {
	return "judgify-security-email/" + deliveryID.String()
}
