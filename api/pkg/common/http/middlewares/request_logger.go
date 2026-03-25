package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/huynhanx03/judgify/global"
	"github.com/huynhanx03/judgify/pkg/constraints"
	"github.com/huynhanx03/judgify/pkg/logger"
)

const headerRequestID = "X-Request-ID"

// RequestLogger logs every HTTP request and injects a per-request logger into context.
// Each request gets a unique request_id for end-to-end tracing.
func RequestLogger() gin.HandlerFunc {
	base := global.LoggerZap.Named("http")

	return func(c *gin.Context) {
		start := time.Now()

		// Generate or reuse request ID
		requestID := c.GetHeader(headerRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Header(headerRequestID, requestID)

		// Build per-request logger with request_id baked in
		reqLogger := base.With(zap.String("request_id", requestID))

		// Inject logger into context — downstream can use logger.FromContext(ctx)
		ctx := logger.WithContext(c.Request.Context(), reqLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// Log after request completes
		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		}

		if query := c.Request.URL.RawQuery; query != "" {
			fields = append(fields, zap.String("query", query))
		}

		if userID, ok := c.Request.Context().Value(constraints.ContextKeyUserID).(int); ok {
			fields = append(fields, zap.Int("user_id", userID))
		}

		if status >= 500 {
			reqLogger.Error("request", fields...)
		} else if status >= 400 {
			reqLogger.Warn("request", fields...)
		} else {
			reqLogger.Info("request", fields...)
		}
	}
}
