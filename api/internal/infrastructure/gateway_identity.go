package infrastructure

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
)

const (
	// HeaderRequestID is minted by the public Envoy listener for every HTTP attempt.
	HeaderRequestID = "X-Request-ID"

	GatewayIdentityLegacy = "legacy"
	GatewayIdentityEnvoy  = "envoy"
)

type requestIDContextKey struct{}

// RequestIDFromContext returns the HTTP-attempt identifier. It is deliberately
// distinct from the durable correlation ID stored by go-common.
func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

// GatewayIdentity accepts only the Envoy-owned identity pair in strict mode.
// Legacy mode exists for local development until its direct server path is
// removed; production Compose always selects envoy mode.
func GatewayIdentity(mode string) gin.HandlerFunc {
	strict := strings.EqualFold(strings.TrimSpace(mode), GatewayIdentityEnvoy)
	return func(c *gin.Context) {
		if !strict {
			c.Next()
			return
		}

		requestID, ok := exactlyOneHeader(c.Request.Header.Values(HeaderRequestID))
		if !ok || !validGatewayUUID(requestID) {
			gatewayIdentityFailure(c)
			return
		}
		correlationID, ok := exactlyOneHeader(c.Request.Header.Values(correlation.Header))
		if !ok || !validGatewayUUID(correlationID) || requestID != correlationID {
			gatewayIdentityFailure(c)
			return
		}

		ctx := correlation.WithContext(c.Request.Context(), correlationID)
		ctx = context.WithValue(ctx, requestIDContextKey{}, requestID)
		ctx = commonlogger.WithFields(ctx, commonlogger.String("request_id", requestID))
		c.Request = c.Request.WithContext(ctx)
		c.Header(HeaderRequestID, requestID)
		c.Header(correlation.Header, correlationID)
		c.Next()
	}
}

func exactlyOneHeader(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	return value, value != ""
}

func validGatewayUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

func gatewayIdentityFailure(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
		"error": gin.H{"code": "invalid_gateway_identity"},
	})
}
