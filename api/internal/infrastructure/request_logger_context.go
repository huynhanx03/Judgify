package infrastructure

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	commonlogger "github.com/huynhanx03/judgify/internal/platform/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const httpCompletedMessage = "HTTP request completed"

// requestLoggerContext keeps the request logger available to handlers. Envoy
// owns public HTTP completion logs; direct development traffic is logged here.
func requestLoggerContext(root *commonlogger.Logger, logCompletion bool) gin.HandlerFunc {
	base := root.Named("http")
	return func(c *gin.Context) {
		ctx := commonlogger.WithLogger(c.Request.Context(), base)
		c.Request = c.Request.WithContext(ctx)
		if !logCompletion {
			c.Next()
			return
		}
		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		level := zapcore.InfoLevel
		switch {
		case status >= http.StatusInternalServerError:
			level = zapcore.ErrorLevel
		case status == http.StatusTooManyRequests:
			level = zapcore.WarnLevel
		}
		durationMS := time.Since(started).Milliseconds()
		commonlogger.Event(c.Request.Context(), level, httpCompletedMessage, &durationMS,
			zap.String("method", c.Request.Method),
			zap.String("route", route),
			zap.Int("status", status),
		)
	}
}
