// Package correlation owns Judgify's internal correlation ID generation. The
// context storage and validation are provided by go-common's logger package.
package correlation

import (
	"context"

	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/logger"
)

const Header = logger.RequestIDHeader

// New returns a time-ordered correlation ID, falling back to UUIDv4 if system
// entropy cannot produce UUIDv7.
func New() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

func WithContext(ctx context.Context, id string) context.Context {
	return logger.WithCorrelationID(ctx, id)
}

func FromContext(ctx context.Context) string { return logger.CorrelationID(ctx) }

func Validate(id string) error { return logger.ValidateRequestID(id) }

// EnsureContext preserves an existing request or correlation ID and generates
// one only for internal work that has no transport identity.
func EnsureContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger.RequestID(ctx) != "" || logger.CorrelationID(ctx) != "" {
		return ctx
	}
	return WithContext(ctx, New())
}
