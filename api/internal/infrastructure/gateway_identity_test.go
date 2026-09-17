package infrastructure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/correlation"
)

const gatewayIdentityUUID = "019f6abb-8dd5-7581-8449-2b9ad77873e5"

func TestGatewayIdentityEnvoyAcceptsOneMatchingUUIDPair(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GatewayIdentity(GatewayIdentityEnvoy))
	router.GET("/", func(c *gin.Context) {
		if got := correlation.FromContext(c.Request.Context()); got != gatewayIdentityUUID {
			t.Fatalf("correlation ID = %q, want %q", got, gatewayIdentityUUID)
		}
		if got := RequestIDFromContext(c.Request.Context()); got != gatewayIdentityUUID {
			t.Fatalf("request ID = %q, want %q", got, gatewayIdentityUUID)
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(HeaderRequestID, gatewayIdentityUUID)
	request.Header.Set(correlation.Header, gatewayIdentityUUID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get(HeaderRequestID); got != gatewayIdentityUUID {
		t.Fatalf("response request ID = %q", got)
	}
	if got := recorder.Header().Get(correlation.Header); got != gatewayIdentityUUID {
		t.Fatalf("response correlation ID = %q", got)
	}
}

func TestGatewayIdentityEnvoyRejectsMissingMalformedOrUnequalHeaders(t *testing.T) {
	for name, headers := range map[string][]string{
		"missing":   {},
		"malformed": {"bad", "bad"},
		"unequal":   {gatewayIdentityUUID, "019f6abb-8dd5-7581-8449-2b9ad77873e6"},
	} {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(GatewayIdentity(GatewayIdentityEnvoy))
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if len(headers) == 2 {
				request.Header.Set(HeaderRequestID, headers[0])
				request.Header.Set(correlation.Header, headers[1])
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestGatewayIdentityLegacyDoesNotRejectDirectDevelopmentRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GatewayIdentity(GatewayIdentityLegacy))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
