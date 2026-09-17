// Package observabilityhttp owns product HTTP metrics instrumentation. The
// transport-neutral metrics catalog remains reusable by API and workers.
package observabilityhttp

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	commonmetrics "github.com/huynhanx03/go-common/pkg/common/observability"

	"github.com/huynhanx03/judgify/internal/platform/observability"
)

// Middleware records only the Gin route template. It never labels metrics
// with the raw path, query, CID, principal, IP address, or request body.
func Middleware(metrics *observability.Metrics, role string) gin.HandlerFunc {
	return func(request *gin.Context) {
		startedAt := time.Now()
		request.Next()

		route := request.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := request.Request.Method
		if !oneOf(method, "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS") {
			method = "OTHER"
		}
		labels := commonmetrics.Labels{
			"role": role, "method": method, "route": route,
			"status_class": statusClass(request.Writer.Status()),
		}
		ctx := request.Request.Context()
		// Client disconnects and handler deadlines are still completed requests.
		// Preserve their CID-bearing values while detaching cancellation so the
		// terminal sample cannot disappear from the latency/error budget.
		if ctx.Err() != nil {
			ctx = context.WithoutCancel(ctx)
		}
		_ = metrics.Inc(ctx, observability.MetricHTTPRequests, labels)
		_ = metrics.Observe(ctx, observability.MetricHTTPRequestDuration, time.Since(startedAt).Seconds(), labels)
	}
}

func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
