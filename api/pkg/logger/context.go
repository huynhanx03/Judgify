package logger

import (
	"context"

	"go.uber.org/zap"
)

type ctxKey struct{}

// WithContext stores a zap.Logger in the context.
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the zap.Logger from context.
// Falls back to the global logger (zap.L()) if not set.
func FromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok {
		return l
	}
	return zap.L()
}
