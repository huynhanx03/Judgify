package operation

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxCursorBytes = 160

func EncodeCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	if createdAt.IsZero() || id == uuid.Nil {
		return "", ErrInvalidOperation
	}
	raw := strconv.FormatInt(createdAt.UTC().UnixNano(), 10) + "." + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func DecodeCursor(value string) (time.Time, uuid.UUID, error) {
	if value == "" {
		return time.Time{}, uuid.Nil, nil
	}
	if len(value) > maxCursorBytes {
		return time.Time{}, uuid.Nil, ErrInvalidOperation
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) > maxCursorBytes {
		return time.Time{}, uuid.Nil, ErrInvalidOperation
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, ErrInvalidOperation
	}
	nanoseconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || nanoseconds <= 0 {
		return time.Time{}, uuid.Nil, ErrInvalidOperation
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, errors.Join(ErrInvalidOperation, err)
	}
	return time.Unix(0, nanoseconds).UTC(), id, nil
}
