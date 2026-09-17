package db

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
)

const campaignCursorBytes = 24

func EncodeCampaignCursor(value ports.CampaignCursor) (string, error) {
	if value.CreatedAt.IsZero() || value.ID == uuid.Nil {
		return "", ports.ErrInvalidCursor
	}
	encoded := make([]byte, campaignCursorBytes)
	binary.BigEndian.PutUint64(encoded[:8], uint64(value.CreatedAt.UTC().UnixNano()))
	copy(encoded[8:], value.ID[:])
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func DecodeCampaignCursor(value string) (ports.CampaignCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != campaignCursorBytes {
		return ports.CampaignCursor{}, ports.ErrInvalidCursor
	}
	var id uuid.UUID
	copy(id[:], decoded[8:])
	createdAt := time.Unix(0, int64(binary.BigEndian.Uint64(decoded[:8]))).UTC()
	if createdAt.IsZero() || id == uuid.Nil {
		return ports.CampaignCursor{}, ports.ErrInvalidCursor
	}
	return ports.CampaignCursor{CreatedAt: createdAt, ID: id}, nil
}

func EncodeRecipientCursor(value ports.RecipientCursor) (string, error) {
	if value.UserID == uuid.Nil {
		return "", ports.ErrInvalidCursor
	}
	return base64.RawURLEncoding.EncodeToString(value.UserID[:]), nil
}

func DecodeRecipientCursor(value string) (ports.RecipientCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != len(uuid.UUID{}) {
		return ports.RecipientCursor{}, ports.ErrInvalidCursor
	}
	var id uuid.UUID
	copy(id[:], decoded)
	if id == uuid.Nil {
		return ports.RecipientCursor{}, ports.ErrInvalidCursor
	}
	return ports.RecipientCursor{UserID: id}, nil
}
