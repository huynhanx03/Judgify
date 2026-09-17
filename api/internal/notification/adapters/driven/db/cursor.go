package db

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ports"
)

const notificationCursorBytes = 24

type notificationCursor struct {
	createdAt time.Time
	id        uuid.UUID
}

func encodeNotificationCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	if createdAt.IsZero() || id == uuid.Nil {
		return "", ports.ErrInvalidCursor
	}
	encoded := make([]byte, notificationCursorBytes)
	binary.BigEndian.PutUint64(encoded[:8], uint64(createdAt.UTC().UnixNano()))
	copy(encoded[8:], id[:])
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeNotificationCursor(value string) (notificationCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != notificationCursorBytes {
		return notificationCursor{}, ports.ErrInvalidCursor
	}
	var id uuid.UUID
	copy(id[:], decoded[8:])
	createdAt := time.Unix(0, int64(binary.BigEndian.Uint64(decoded[:8]))).UTC()
	if createdAt.IsZero() || id == uuid.Nil {
		return notificationCursor{}, ports.ErrInvalidCursor
	}
	return notificationCursor{createdAt: createdAt, id: id}, nil
}
