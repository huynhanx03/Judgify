package usecase

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

const (
	campaignConfirmationNonceBytes = 32
	campaignConfirmationMACBytes   = sha256.Size
	campaignConfirmationTokenBytes = campaignConfirmationNonceBytes + campaignConfirmationMACBytes
)

var ErrInvalidCampaignConfirmationProtector = errors.New("notification campaign: invalid confirmation protector")

// CampaignConfirmationProtector authenticates the random single-use token
// before any database lookup. Only SHA-256(token) is persisted.
type CampaignConfirmationProtector struct {
	key    [sha256.Size]byte
	random io.Reader
}

func NewCampaignConfirmationProtector(secret []byte) (*CampaignConfirmationProtector, error) {
	if len(secret) < sha256.Size {
		return nil, ErrInvalidCampaignConfirmationProtector
	}
	key := sha256.Sum256(append(
		[]byte("judgify.notification-campaign-confirmation.v1\x00"),
		secret...,
	))
	return &CampaignConfirmationProtector{key: key, random: rand.Reader}, nil
}

func (protector *CampaignConfirmationProtector) Issue() (string, [sha256.Size]byte, error) {
	if protector == nil || protector.random == nil || protector.key == ([sha256.Size]byte{}) {
		return "", [sha256.Size]byte{}, ErrInvalidCampaignConfirmationProtector
	}
	token := make([]byte, campaignConfirmationTokenBytes)
	if _, err := io.ReadFull(protector.random, token[:campaignConfirmationNonceBytes]); err != nil {
		return "", [sha256.Size]byte{}, ErrInvalidCampaignConfirmationProtector
	}
	mac := hmac.New(sha256.New, protector.key[:])
	_, _ = mac.Write([]byte("confirm\x00"))
	_, _ = mac.Write(token[:campaignConfirmationNonceBytes])
	copy(token[campaignConfirmationNonceBytes:], mac.Sum(nil))
	encoded := base64.RawURLEncoding.EncodeToString(token)
	return encoded, sha256.Sum256([]byte(encoded)), nil
}

func (protector *CampaignConfirmationProtector) Hash(
	encoded string,
) ([sha256.Size]byte, error) {
	if protector == nil || protector.key == ([sha256.Size]byte{}) {
		return [sha256.Size]byte{}, ErrInvalidCampaignConfirmationProtector
	}
	token, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(token) != campaignConfirmationTokenBytes {
		return [sha256.Size]byte{}, ErrInvalidCampaignConfirmationProtector
	}
	mac := hmac.New(sha256.New, protector.key[:])
	_, _ = mac.Write([]byte("confirm\x00"))
	_, _ = mac.Write(token[:campaignConfirmationNonceBytes])
	if !hmac.Equal(token[campaignConfirmationNonceBytes:], mac.Sum(nil)) {
		return [sha256.Size]byte{}, ErrInvalidCampaignConfirmationProtector
	}
	return sha256.Sum256([]byte(encoded)), nil
}
