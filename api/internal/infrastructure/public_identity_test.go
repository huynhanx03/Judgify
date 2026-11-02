package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/judgify/internal/authctx"
)

type publicIdentityVerifierFunc func(context.Context, string, authentication.TokenPurpose) (authentication.Principal, error)

func (fn publicIdentityVerifierFunc) Verify(
	ctx context.Context,
	raw string,
	purpose authentication.TokenPurpose,
) (authentication.Principal, error) {
	return fn(ctx, raw, purpose)
}

func TestPublicContentIdentityIsAnonymousOnlyWhenCredentialIsAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	var verifierCalls atomic.Int32
	verifier := publicIdentityVerifierFunc(func(
		_ context.Context,
		raw string,
		purpose authentication.TokenPurpose,
	) (authentication.Principal, error) {
		verifierCalls.Add(1)
		if purpose != authentication.PurposeAccess || raw != "valid-access" {
			return authentication.Anonymous(), authentication.ErrInvalidToken
		}
		return authentication.Principal{
			Authenticated:      true,
			Subject:            userID.String(),
			UserID:             userID.String(),
			SessionID:          sessionID.String(),
			TokenID:            tokenID.String(),
			AuthenticatedAt:    time.Now().UTC().Add(-time.Minute),
			SecurityVersion:    1,
			AuthenticatedUntil: time.Now().UTC().Add(time.Hour),
		}, nil
	})

	router := gin.New()
	public := router.Group("/")
	content := publicContentWithOptionalIdentity(public, verifier, "judgify_access", 4096)
	content.GET("/problem", func(c *gin.Context) {
		id, ok := authctx.UserID(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"authenticated": ok, "user_id": id.String()})
	})
	public.GET("/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/problem", nil))
	if anonymous.Code != http.StatusOK || anonymous.Body.String() != "{\"authenticated\":false,\"user_id\":\"00000000-0000-0000-0000-000000000000\"}" {
		t.Fatalf("anonymous public content status=%d body=%s", anonymous.Code, anonymous.Body.String())
	}
	if verifierCalls.Load() != 0 {
		t.Fatal("verifier called without a credential")
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/problem", nil)
	authenticatedRequest.Header.Set("Authorization", "Bearer valid-access")
	authenticated := httptest.NewRecorder()
	router.ServeHTTP(authenticated, authenticatedRequest)
	if authenticated.Code != http.StatusOK || authenticated.Body.String() != "{\"authenticated\":true,\"user_id\":\""+userID.String()+"\"}" {
		t.Fatalf("authenticated public content status=%d body=%s", authenticated.Code, authenticated.Body.String())
	}
	if verifierCalls.Load() != 1 {
		t.Fatalf("verifier calls=%d, want 1", verifierCalls.Load())
	}

	invalidRequest := httptest.NewRequest(http.MethodGet, "/problem", nil)
	invalidRequest.Header.Set("Authorization", "Bearer invalid")
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, invalidRequest)
	if invalid.Code != http.StatusUnauthorized {
		t.Fatalf("invalid credential status=%d, want 401", invalid.Code)
	}

	// Authentication/session-establishment endpoints intentionally remain
	// outside optional access-token verification. A stale bearer must not make
	// login or registration unreachable.
	loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginRequest.Header.Set("Authorization", "Bearer invalid")
	login := httptest.NewRecorder()
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusNoContent {
		t.Fatalf("login boundary status=%d, want 204", login.Code)
	}
}

func TestPublicContentDoesNotInheritAuthenticationRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var authenticationPolicyCalls atomic.Int32
	authenticationPolicy := func(c *gin.Context) {
		authenticationPolicyCalls.Add(1)
		c.Next()
	}
	verifier := publicIdentityVerifierFunc(func(
		context.Context,
		string,
		authentication.TokenPurpose,
	) (authentication.Principal, error) {
		return authentication.Anonymous(), authentication.ErrInvalidToken
	})

	publicAuthentication, publicContent := publicRouteBoundaries(
		router,
		authenticationPolicy,
		verifier,
		"judgify_access",
		4096,
	)
	publicAuthentication.GET("/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	publicContent.GET("/problems", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/problems", nil))
	if got := authenticationPolicyCalls.Load(); got != 0 {
		t.Fatalf("authentication policy calls for public content = %d, want 0", got)
	}
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/login", nil))
	if got := authenticationPolicyCalls.Load(); got != 1 {
		t.Fatalf("authentication policy calls after login = %d, want 1", got)
	}
}
