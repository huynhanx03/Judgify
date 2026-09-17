package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

type sessionManagementHandlerStub struct {
	list      func(context.Context, *sessionListQuery) (*sessionListResponse, error)
	revoke    func(context.Context, uuid.UUID, uuid.UUID) error
	logoutAll func(context.Context, uuid.UUID) error
}

func (stub sessionManagementHandlerStub) List(ctx context.Context, request *sessionListQuery) (*sessionListResponse, error) {
	return stub.list(ctx, request)
}

func (stub sessionManagementHandlerStub) Revoke(ctx context.Context, targetID, idempotencyKey uuid.UUID) error {
	return stub.revoke(ctx, targetID, idempotencyKey)
}

func (stub sessionManagementHandlerStub) LogoutAll(ctx context.Context, idempotencyKey uuid.UUID) error {
	return stub.logoutAll(ctx, idempotencyKey)
}

func TestSessionManagementRoutesUseCanonicalAuthAndSafeHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New()
	currentSessionID := uuid.New()
	targetSessionID := uuid.New()
	listRequest := (*sessionListQuery)(nil)
	var revokedTarget, revokedKey uuid.UUID
	var logoutKey, receivedLogoutKey uuid.UUID
	identity := &IdentityHandler{
		SessionManagementHandler: sessionManagementHandlerStub{
			list: func(_ context.Context, request *sessionListQuery) (*sessionListResponse, error) {
				listRequest = request
				return &sessionListResponse{Items: []sessionResponse{{ID: currentSessionID.String(), Current: true, Status: string(entity.SessionStatusActive)}}, NextCursor: "next"}, nil
			},
			revoke: func(_ context.Context, target, key uuid.UUID) error {
				revokedTarget, revokedKey = target, key
				return nil
			},
			logoutAll: func(_ context.Context, key uuid.UUID) error {
				receivedLogoutKey = key
				return nil
			},
		},
		BrowserSession: testBrowserSessionConfig(time.Now().UTC()),
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authenticatedSessionContext(userID, currentSessionID, now))
		c.Next()
	})
	identity.registerSessionManagementRoutes(router.Group(""))

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/auth/sessions?limit=7&cursor=opaque", nil))
	require.Equal(t, http.StatusOK, listRecorder.Code, listRecorder.Body.String())
	require.NotNil(t, listRequest)
	require.Equal(t, 7, listRequest.Limit)
	require.Equal(t, "opaque", listRequest.Cursor)
	require.Equal(t, "no-store", listRecorder.Header().Get("Cache-Control"))

	key := uuid.New()
	request := httptest.NewRequest(http.MethodDelete, "/auth/sessions/"+targetSessionID.String(), nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Idempotency-Key", key.String())
	revokeRecorder := httptest.NewRecorder()
	router.ServeHTTP(revokeRecorder, request)
	require.Equal(t, http.StatusOK, revokeRecorder.Code, revokeRecorder.Body.String())
	require.Equal(t, targetSessionID, revokedTarget)
	require.Equal(t, key, revokedKey)
	require.Empty(t, revokeRecorder.Header().Get("Set-Cookie"))

	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout-all", strings.NewReader(""))
	logoutRequest.Header.Set("Origin", "https://app.example")
	logoutRequest.Header.Set("Sec-Fetch-Site", "same-site")
	logoutKey = uuid.New()
	logoutRequest.Header.Set("Idempotency-Key", logoutKey.String())
	logoutRecorder := httptest.NewRecorder()
	router.ServeHTTP(logoutRecorder, logoutRequest)
	require.Equal(t, http.StatusOK, logoutRecorder.Code, logoutRecorder.Body.String())
	require.Equal(t, logoutKey, receivedLogoutKey)
	require.NotNil(t, responseCookieMap(logoutRecorder)[identity.BrowserSession.Refresh.Name])
}

func TestSessionManagementMutationRoutesRejectUntrustedBrowserAndInvalidTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	identity := &IdentityHandler{
		SessionManagementHandler: sessionManagementHandlerStub{
			revoke:    func(context.Context, uuid.UUID, uuid.UUID) error { t.Fatal("revoke must not run"); return nil },
			logoutAll: func(context.Context, uuid.UUID) error { t.Fatal("logout-all must not run"); return nil },
		},
		BrowserSession: testBrowserSessionConfig(time.Now().UTC()),
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authenticatedSessionContext(uuid.New(), uuid.New(), time.Now().UTC()))
		c.Next()
	})
	identity.registerSessionManagementRoutes(router.Group(""))

	untrusted := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/logout-all", nil)
	request.Header.Set("Idempotency-Key", uuid.NewString())
	router.ServeHTTP(untrusted, request)
	require.Equal(t, http.StatusForbidden, untrusted.Code, untrusted.Body.String())

	invalid := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/auth/sessions/not-a-uuid", nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Idempotency-Key", uuid.NewString())
	router.ServeHTTP(invalid, request)
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
}

func authenticatedSessionContext(userID, sessionID uuid.UUID, now time.Time) context.Context {
	return authentication.WithPrincipal(context.Background(), authentication.Principal{
		Authenticated: true, Subject: userID.String(), UserID: userID.String(), SessionID: sessionID.String(),
		TokenID: uuid.NewString(), AuthenticatedAt: now.Add(-time.Minute), AuthenticatedUntil: now.Add(time.Hour), SecurityVersion: 1,
	})
}
