package usecase

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"crypto/hkdf"
)

const (
	refreshTokenPrefix  = "jrf1."
	refreshEntropyBytes = 32
	derivedKeyBytes     = 32
)

var errInvalidProtectedCredential = errors.New("identity: invalid protected credential")

type tokenProtector struct {
	hashKey        []byte
	fingerprintKey []byte
	aead           cipher.AEAD
	random         io.Reader
}

func newTokenProtector(secret []byte, random io.Reader) (*tokenProtector, error) {
	if len(secret) < derivedKeyBytes || len(secret) > 4096 || random == nil {
		return nil, fmt.Errorf("identity token protection: invalid key material")
	}
	refreshKey, err := deriveIdentityKey(secret, "judgify/identity/refresh-hmac/v1")
	if err != nil {
		return nil, fmt.Errorf("derive refresh hash key: %w", err)
	}
	fingerprintKey, err := deriveIdentityKey(secret, "judgify/identity/request-fingerprint/v1")
	if err != nil {
		return nil, fmt.Errorf("derive fingerprint key: %w", err)
	}
	recoveryKey, err := deriveIdentityKey(secret, "judgify/identity/recovery-aead/v1")
	if err != nil {
		return nil, fmt.Errorf("derive recovery key: %w", err)
	}
	aead, err := newIdentityAEAD(recoveryKey)
	if err != nil {
		return nil, fmt.Errorf("initialize recovery AEAD: %w", err)
	}
	return &tokenProtector{
		hashKey:        refreshKey,
		fingerprintKey: fingerprintKey,
		aead:           aead,
		random:         random,
	}, nil
}

func deriveIdentityKey(secret []byte, label string) ([]byte, error) {
	if len(secret) < derivedKeyBytes || len(secret) > 4096 || label == "" {
		return nil, errInvalidProtectedCredential
	}
	return hkdf.Key(sha256.New, secret, nil, label, derivedKeyBytes)
}

func newIdentityAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (protector *tokenProtector) newRefreshToken() (string, []byte, error) {
	if protector == nil || protector.random == nil {
		return "", nil, errInvalidProtectedCredential
	}
	entropy := make([]byte, refreshEntropyBytes)
	if _, err := io.ReadFull(protector.random, entropy); err != nil {
		return "", nil, fmt.Errorf("generate refresh credential: %w", err)
	}
	raw := refreshTokenPrefix + base64.RawURLEncoding.EncodeToString(entropy)
	return raw, protector.hashRefresh(raw), nil
}

func (protector *tokenProtector) parseAndHashRefresh(raw string, maximumBytes int) ([]byte, error) {
	if protector == nil || len(raw) > maximumBytes || !strings.HasPrefix(raw, refreshTokenPrefix) ||
		len(raw) != len(refreshTokenPrefix)+base64.RawURLEncoding.EncodedLen(refreshEntropyBytes) {
		return nil, errInvalidProtectedCredential
	}
	encoded := strings.TrimPrefix(raw, refreshTokenPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != refreshEntropyBytes {
		return nil, errInvalidProtectedCredential
	}
	for _, character := range raw {
		if character <= 0x20 || character == 0x7f {
			return nil, errInvalidProtectedCredential
		}
	}
	return protector.hashRefresh(raw), nil
}

func (protector *tokenProtector) hashRefresh(raw string) []byte {
	mac := hmac.New(sha256.New, protector.hashKey)
	_, _ = mac.Write([]byte(raw))
	return mac.Sum(nil)
}

func (protector *tokenProtector) fingerprint(material []byte) []byte {
	mac := hmac.New(sha256.New, protector.fingerprintKey)
	_, _ = mac.Write(material)
	return mac.Sum(nil)
}

func (protector *tokenProtector) seal(plaintext, associatedData []byte) ([]byte, error) {
	if protector == nil || len(plaintext) == 0 || len(plaintext) > 64<<10 || len(associatedData) == 0 {
		return nil, errInvalidProtectedCredential
	}
	nonce := make([]byte, protector.aead.NonceSize())
	if _, err := io.ReadFull(protector.random, nonce); err != nil {
		return nil, fmt.Errorf("generate recovery nonce: %w", err)
	}
	result := make([]byte, 0, len(nonce)+len(plaintext)+protector.aead.Overhead())
	result = append(result, nonce...)
	return protector.aead.Seal(result, nonce, plaintext, associatedData), nil
}

func (protector *tokenProtector) open(ciphertext, associatedData []byte) ([]byte, error) {
	if protector == nil || len(associatedData) == 0 ||
		len(ciphertext) <= protector.aead.NonceSize()+protector.aead.Overhead() || len(ciphertext) > 128<<10 {
		return nil, errInvalidProtectedCredential
	}
	nonceSize := protector.aead.NonceSize()
	plaintext, err := protector.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], associatedData)
	if err != nil {
		return nil, errInvalidProtectedCredential
	}
	return plaintext, nil
}

func defaultCryptoRandom() io.Reader { return rand.Reader }
