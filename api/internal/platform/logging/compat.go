// Package logger adapts Judgify's existing context-based logging boundaries to
// go-common's context-aware logger.
package logger

import (
	"context"

	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger = commonlogger.Logger
type LoggerConfig = commonlogger.LoggerConfig
type Field = commonlogger.Field

const RequestIDHeader = commonlogger.RequestIDHeader

type loggerContextKey struct{}

var noOpLogger = commonlogger.Wrap(zap.NewNop())

// NewLogger creates the current go-common logger for process dependency
// injection.
func NewLogger(config LoggerConfig) (*Logger, error) {
	return commonlogger.NewLogger(config)
}

// Wrap adapts a Zap logger to go-common's context-aware API.
func Wrap(base *zap.Logger) *Logger { return commonlogger.Wrap(base) }

// Zap returns a Zap logger for dependencies that have not adopted the shared
// logger interface yet.
func Zap(base *Logger) *zap.Logger {
	return zap.New(base.Core(), zap.AddCaller(), zap.AddCallerSkip(1))
}

// WithLogger binds the injected logger to a context at an application edge.
func WithLogger(ctx context.Context, base *Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerContextKey{}, base)
}

// WithContext keeps compatibility with boundaries that still own a Zap
// logger directly.
func WithContext(ctx context.Context, base *zap.Logger) context.Context {
	return WithLogger(ctx, Wrap(base))
}

// ContextLogger binds one context to go-common's per-call logger methods.
type ContextLogger struct {
	base *Logger
	ctx  context.Context
}

// FromContext returns the logger bound at the nearest process or transport
// boundary. A no-op logger keeps standalone/test contexts safe.
func FromContext(ctx context.Context) *ContextLogger {
	if ctx == nil {
		ctx = context.Background()
	}
	base, _ := ctx.Value(loggerContextKey{}).(*Logger)
	if base == nil {
		base = noOpLogger
	}
	return &ContextLogger{base: base, ctx: ctx}
}

func (log *ContextLogger) Debug(message string, fields ...zap.Field) {
	log.base.Debug(log.ctx, message, fields...)
}

func (log *ContextLogger) Info(message string, fields ...zap.Field) {
	log.base.Info(log.ctx, message, fields...)
}

func (log *ContextLogger) Warn(message string, fields ...zap.Field) {
	log.base.Warn(log.ctx, message, fields...)
}

func (log *ContextLogger) Error(message string, fields ...zap.Field) {
	log.base.Error(log.ctx, message, fields...)
}

func (log *ContextLogger) With(fields ...zap.Field) *ContextLogger {
	return &ContextLogger{base: log.base.With(fields...), ctx: log.ctx}
}

func (log *ContextLogger) Named(name string) *ContextLogger {
	return &ContextLogger{base: log.base.Named(name), ctx: log.ctx}
}

func (log *ContextLogger) Event(level zapcore.Level, message string, durationMS *int64, fields ...zap.Field) {
	log.base.Event(log.ctx, level, message, durationMS, fields...)
}

func Event(ctx context.Context, level zapcore.Level, message string, durationMS *int64, fields ...zap.Field) {
	FromContext(ctx).Event(level, message, durationMS, fields...)
}

func WithFields(ctx context.Context, fields ...Field) context.Context {
	return commonlogger.WithFields(ctx, fields...)
}

func String(key, value string) Field { return commonlogger.String(key, value) }

func WithRequestID(ctx context.Context, id string) context.Context {
	return commonlogger.WithRequestID(ctx, id)
}

func RequestID(ctx context.Context) string { return commonlogger.RequestID(ctx) }

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return commonlogger.WithCorrelationID(ctx, id)
}

func CorrelationID(ctx context.Context) string { return commonlogger.CorrelationID(ctx) }

func ValidateRequestID(id string) error { return commonlogger.ValidateRequestID(id) }
