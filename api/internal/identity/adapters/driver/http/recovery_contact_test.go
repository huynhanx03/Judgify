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
	"github.com/huynhanx03/judgify/internal/ports"
)

type recoveryContactHandlerStub struct {
	RecoveryContactHandler
	verify         func(context.Context, *verifyRecoveryContactRequest) (*verifyRecoveryContactResponse, error)
	get            func(context.Context) (*recoveryContactResponse, error)
	reauthenticate func(context.Context, *reauthenticateRecoveryContactRequest) (*reauthenticateRecoveryContactResponse, error)
	replace        func(context.Context, *replaceRecoveryContactRequest) (*recoveryContactResponse, error)
}

func (stub recoveryContactHandlerStub) Replace(
	ctx context.Context,
	request *replaceRecoveryContactRequest,
) (*recoveryContactResponse, error) {
	return stub.replace(ctx, request)
}

func (stub recoveryContactHandlerStub) Get(ctx context.Context) (*recoveryContactResponse, error) {
	return stub.get(ctx)
}

func (stub recoveryContactHandlerStub) Reauthenticate(
	ctx context.Context,
	request *reauthenticateRecoveryContactRequest,
) (*reauthenticateRecoveryContactResponse, error) {
	return stub.reauthenticate(ctx, request)
}

func (stub recoveryContactHandlerStub) Verify(
	ctx context.Context,
	request *verifyRecoveryContactRequest,
) (*verifyRecoveryContactResponse, error) {
	return stub.verify(ctx, request)
}

type recoveryContactLifecycleManagementStub struct {
	ports.RecoveryContactLifecycle
	projection *entity.RecoveryContactProjection
	userID     uuid.UUID
	sessionID  uuid.UUID
	password   string
	until      time.Time
}

func (stub *recoveryContactLifecycleManagementStub) Replace(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, uint64,
) (*entity.RecoveryContactProjection, error) {
	return stub.projection, nil
}

func (stub *recoveryContactLifecycleManagementStub) Resend(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64,
) (*entity.RecoveryContactProjection, error) {
	return stub.projection, nil
}

func (stub *recoveryContactLifecycleManagementStub) CancelPending(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64,
) (*entity.RecoveryContactProjection, error) {
	return stub.projection, nil
}

func (stub *recoveryContactLifecycleManagementStub) Remove(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uint64,
) (*entity.RecoveryContactProjection, error) {
	return stub.projection, nil
}

func (stub *recoveryContactLifecycleManagementStub) Get(
	_ context.Context,
	userID, sessionID uuid.UUID,
) (*entity.RecoveryContactProjection, error) {
	stub.userID = userID
	stub.sessionID = sessionID
	return stub.projection, nil
}

func (stub *recoveryContactLifecycleManagementStub) Reauthenticate(
	_ context.Context,
	userID, sessionID uuid.UUID,
	password string,
) (time.Time, error) {
	stub.userID = userID
	stub.sessionID = sessionID
	stub.password = password
	return stub.until, nil
}

func TestRecoveryContactManagementHandlerUsesCanonicalPrincipalAndSafeProjection(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New()
	sessionID := uuid.New()
	until := now.Add(10 * time.Minute)
	lifecycle := &recoveryContactLifecycleManagementStub{
		until: until,
		projection: &entity.RecoveryContactProjection{
			CanRemove: true,
			Current: &entity.RecoveryContactView{
				Status: entity.RecoveryContactStatusVerified, Purpose: entity.RecoveryContactPurposeRegistration,
				MaskedAddress: "o***@example.com", Version: 2,
			},
			ReauthenticatedUntil: &until,
		},
	}
	handler := NewRecoveryContactHandler(lifecycle)
	ctx := authenticatedRecoveryContactContext(userID, sessionID, now)

	projection, err := handler.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, userID, lifecycle.userID)
	require.Equal(t, sessionID, lifecycle.sessionID)
	require.Equal(t, "o***@example.com", projection.Current.MaskedAddress)
	require.True(t, projection.CanRemove)

	result, err := handler.Reauthenticate(ctx, &reauthenticateRecoveryContactRequest{
		CurrentPassword: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.Equal(t, "correct horse battery staple", lifecycle.password)
	require.Equal(t, reauthenticationStatusVerified, result.Status)
	require.Equal(t, until, result.ReauthenticatedUntil)
}

func TestRecoveryContactProtectedRoutesAreNoStoreAndMutationIsOriginBound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	userID := uuid.New()
	sessionID := uuid.New()
	until := now.Add(10 * time.Minute)
	var reauthenticated bool
	var replacement *replaceRecoveryContactRequest
	identity := &IdentityHandler{
		RecoveryContactHandler: recoveryContactHandlerStub{
			get: func(context.Context) (*recoveryContactResponse, error) {
				return &recoveryContactResponse{}, nil
			},
			replace: func(_ context.Context, request *replaceRecoveryContactRequest) (*recoveryContactResponse, error) {
				replacement = request
				return &recoveryContactResponse{}, nil
			},
			reauthenticate: func(
				_ context.Context,
				request *reauthenticateRecoveryContactRequest,
			) (*reauthenticateRecoveryContactResponse, error) {
				reauthenticated = request.CurrentPassword == "password-for-test"
				return &reauthenticateRecoveryContactResponse{
					Status: reauthenticationStatusVerified, ReauthenticatedUntil: until,
				}, nil
			},
		},
		BrowserSession: testBrowserSessionConfig(time.Now().UTC()),
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authenticatedRecoveryContactContext(userID, sessionID, now))
		c.Next()
	})
	identity.registerRecoveryContactRoutes(router.Group(""))

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/auth/recovery-contact", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code, getRecorder.Body.String())
	require.Equal(t, "no-store", getRecorder.Header().Get("Cache-Control"))

	untrusted := httptest.NewRecorder()
	untrustedRequest := httptest.NewRequest(http.MethodPost, "/auth/reauthenticate", strings.NewReader(`{"current_password":"password-for-test"}`))
	untrustedRequest.Header.Set("Content-Type", "application/json")
	untrustedRequest.Header.Set("Idempotency-Key", uuid.NewString())
	router.ServeHTTP(untrusted, untrustedRequest)
	require.Equal(t, http.StatusForbidden, untrusted.Code, untrusted.Body.String())
	require.False(t, reauthenticated)

	trusted := httptest.NewRecorder()
	trustedRequest := httptest.NewRequest(http.MethodPost, "/auth/reauthenticate", strings.NewReader(`{"current_password":"password-for-test"}`))
	trustedRequest.Header.Set("Content-Type", "application/json")
	trustedRequest.Header.Set("Origin", "https://app.example")
	trustedRequest.Header.Set("Sec-Fetch-Site", "same-site")
	trustedRequest.Header.Set("Idempotency-Key", uuid.NewString())
	router.ServeHTTP(trusted, trustedRequest)
	require.Equal(t, http.StatusOK, trusted.Code, trusted.Body.String())
	require.True(t, reauthenticated)
	require.Equal(t, "no-store", trusted.Header().Get("Cache-Control"))

	replaceRecorder := httptest.NewRecorder()
	replaceKey := uuid.New()
	replaceRequest := httptest.NewRequest(http.MethodPut, "/auth/recovery-contact", strings.NewReader(`{"recovery_email":"next@example.com","expected_current_version":2}`))
	replaceRequest.Header.Set("Content-Type", "application/json")
	replaceRequest.Header.Set("Origin", "https://app.example")
	replaceRequest.Header.Set("Sec-Fetch-Site", "same-site")
	replaceRequest.Header.Set("Idempotency-Key", replaceKey.String())
	router.ServeHTTP(replaceRecorder, replaceRequest)
	require.Equal(t, http.StatusOK, replaceRecorder.Code, replaceRecorder.Body.String())
	require.NotNil(t, replacement)
	require.Equal(t, replaceKey, replacement.IdempotencyKey)
	require.Equal(t, uint64(2), replacement.ExpectedCurrentVersion)
	require.Equal(t, "no-store", replaceRecorder.Header().Get("Cache-Control"))
}

func authenticatedRecoveryContactContext(
	userID, sessionID uuid.UUID,
	now time.Time,
) context.Context {
	return authentication.WithPrincipal(context.Background(), authentication.Principal{
		Authenticated: true, Subject: userID.String(), UserID: userID.String(),
		SessionID: sessionID.String(), TokenID: uuid.NewString(),
		AuthenticatedAt: now.Add(-time.Minute), AuthenticatedUntil: now.Add(time.Hour),
		SecurityVersion: 1,
	})
}

func TestRecoveryContactVerificationRouteNeverEchoesCredentialAndDisablesCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	rawToken := "jcv1." + strings.Repeat("A", 43)
	var received string
	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{},
		BrowserSession:        testBrowserSessionConfig(now),
		RecoveryContactHandler: recoveryContactHandlerStub{verify: func(
			_ context.Context,
			request *verifyRecoveryContactRequest,
		) (*verifyRecoveryContactResponse, error) {
			received = request.Token
			return &verifyRecoveryContactResponse{Status: "verified"}, nil
		}},
	}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/recovery-contact/verify",
		strings.NewReader(`{"token":"`+rawToken+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Empty(t, received)

	bootstrapKey := uuid.New()
	preSession, csrfToken, err := identity.BrowserSession.issuePreSession(bootstrapKey)
	require.NoError(t, err)

	request = httptest.NewRequest(
		http.MethodPost,
		"/auth/recovery-contact/verify",
		strings.NewReader(`{"token":"`+rawToken+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-site")
	request.Header.Set("Idempotency-Key", bootstrapKey.String())
	request.Header.Set(CSRFHeaderName, csrfToken)
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.PreSession.Name, Value: preSession})
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.CSRF.Name, Value: csrfToken})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, rawToken, received)
	require.Contains(t, response.Body.String(), `"status":"verified"`)
	require.NotContains(t, response.Body.String(), rawToken)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
}

func TestRecoveryContactVerificationRouteRejectsUnboundedCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{},
		BrowserSession:        testBrowserSessionConfig(now),
		RecoveryContactHandler: recoveryContactHandlerStub{verify: func(
			context.Context,
			*verifyRecoveryContactRequest,
		) (*verifyRecoveryContactResponse, error) {
			t.Fatal("invalid request reached recovery contact service")
			return nil, nil
		}},
	}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	bootstrapKey := uuid.New()
	preSession, csrfToken, err := identity.BrowserSession.issuePreSession(bootstrapKey)
	require.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/recovery-contact/verify",
		strings.NewReader(`{"token":"`+strings.Repeat("A", 4097)+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-site")
	request.Header.Set("Idempotency-Key", bootstrapKey.String())
	request.Header.Set(CSRFHeaderName, csrfToken)
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.PreSession.Name, Value: preSession})
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.CSRF.Name, Value: csrfToken})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
}
