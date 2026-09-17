package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

type authenticationHandlerStub struct {
	login        func(context.Context, *dto.LoginRequest) (*dto.LoginResponse, error)
	refresh      func(context.Context, *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error)
	logout       func(context.Context) (*dto.LogoutResponse, error)
	forgot       func(context.Context, *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error)
	reset        func(context.Context, *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
	oauthInspect func(context.Context, *dto.OAuthLoginInspectionRequest) (*dto.OAuthLoginInspectionResponse, error)
}

func (authenticationHandlerStub) Register(context.Context, *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	return nil, nil
}

func (stub authenticationHandlerStub) Login(ctx context.Context, request *dto.LoginRequest) (*dto.LoginResponse, error) {
	if stub.login != nil {
		return stub.login(ctx, request)
	}
	return nil, nil
}
func (authenticationHandlerStub) ChangePassword(context.Context, *dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	return nil, nil
}
func (stub authenticationHandlerStub) RefreshToken(ctx context.Context, request *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	if stub.refresh != nil {
		return stub.refresh(ctx, request)
	}
	return nil, nil
}

func (stub authenticationHandlerStub) Logout(ctx context.Context) (*dto.LogoutResponse, error) {
	if stub.logout != nil {
		return stub.logout(ctx)
	}
	return &dto.LogoutResponse{Success: true}, nil
}
func (authenticationHandlerStub) OAuthStart(context.Context, *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error) {
	return nil, nil
}
func (authenticationHandlerStub) OAuthLinkStart(context.Context, *dto.OAuthStartRequest) (*dto.OAuthStartResponse, error) {
	return nil, nil
}
func (authenticationHandlerStub) OAuthCallback(context.Context, *dto.OAuthCallbackRequest) (*dto.OAuthCallbackResponse, error) {
	return nil, nil
}
func (stub authenticationHandlerStub) OAuthInspectLogin(ctx context.Context, request *dto.OAuthLoginInspectionRequest) (*dto.OAuthLoginInspectionResponse, error) {
	if stub.oauthInspect != nil {
		return stub.oauthInspect(ctx, request)
	}
	return nil, nil
}
func (authenticationHandlerStub) OAuthFinalizeLogin(context.Context, *dto.OAuthFinalizeLoginRequest) (*dto.OAuthFinalizeLoginResponse, error) {
	return nil, nil
}
func (authenticationHandlerStub) OAuthFinalizeLink(context.Context, *dto.OAuthFinalizeLinkRequest) (*dto.OAuthFinalizeLinkResponse, error) {
	return nil, nil
}
func (authenticationHandlerStub) OAuthIdentities(context.Context) (*oauthIdentityListResponse, error) {
	return nil, nil
}
func (authenticationHandlerStub) OAuthUnlink(context.Context, *oauthProviderRequest) (*oauthUnlinkResponse, error) {
	return nil, nil
}

func (stub authenticationHandlerStub) ForgotPassword(ctx context.Context, request *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
	if stub.forgot != nil {
		return stub.forgot(ctx, request)
	}
	return nil, nil
}
func (stub authenticationHandlerStub) ResetPassword(ctx context.Context, request *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	if stub.reset != nil {
		return stub.reset(ctx, request)
	}
	return nil, nil
}

func TestRefreshRouteUsesOnlyOpaqueHttpOnlyCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	config := testBrowserSessionConfig(now)
	idempotencyKey := uuid.New()
	sessionID := uuid.New()
	csrf, err := config.issueSessionCSRF(sessionID, now.Add(24*time.Hour))
	require.NoError(t, err)
	var received *dto.RefreshTokenRequest
	handler := authenticationHandlerStub{refresh: func(
		_ context.Context,
		request *dto.RefreshTokenRequest,
	) (*dto.RefreshTokenResponse, error) {
		copied := *request
		received = &copied
		return &dto.RefreshTokenResponse{
			SessionID:             sessionID,
			AccessToken:           "new-access-token",
			AccessTokenExpiresAt:  now.Add(15 * time.Minute),
			RefreshToken:          "new-refresh-token",
			RefreshTokenExpiresAt: now.Add(time.Hour),
		}, nil
	}}

	router := gin.New()
	identity := &IdentityHandler{
		AuthenticationHandler: handler,
		BrowserSession:        config,
	}
	identity.RegisterRefresh(router.Group(""))

	request := refreshRequest(idempotencyKey)
	request.Header.Set(CSRFHeaderName, csrf)
	request.AddCookie(&http.Cookie{Name: config.CSRF.Name, Value: csrf})
	request.AddCookie(&http.Cookie{Name: config.Refresh.Name, Value: "presented-refresh-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NotNil(t, received)
	require.Equal(t, "presented-refresh-token", received.RefreshToken)
	require.Equal(t, idempotencyKey, received.IdempotencyKey)
	require.NotContains(t, response.Body.String(), "new-refresh-token")

	cookies := responseCookieMap(response)
	refreshCookie := cookies[config.Refresh.Name]
	require.NotNil(t, refreshCookie)
	require.Equal(t, "new-refresh-token", refreshCookie.Value)
	require.True(t, refreshCookie.HttpOnly)
	require.Equal(t, http.SameSiteStrictMode, refreshCookie.SameSite)
	require.Equal(t, config.Refresh.Path, refreshCookie.Path)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
}

func TestLoginTransportDerivesBoundedMetadataAndHidesRefreshSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	idempotencyKey := uuid.New()
	sessionID := uuid.New()
	var received *dto.LoginRequest
	handler := authenticationHandlerStub{login: func(
		_ context.Context,
		request *dto.LoginRequest,
	) (*dto.LoginResponse, error) {
		copied := *request
		copied.IPHash = append([]byte(nil), request.IPHash...)
		received = &copied
		return &dto.LoginResponse{
			SessionID:             sessionID,
			AccessToken:           "new-access-token",
			AccessTokenExpiresAt:  now.Add(15 * time.Minute),
			RefreshToken:          "new-refresh-token",
			RefreshTokenExpiresAt: now.Add(time.Hour),
		}, nil
	}}
	identity := &IdentityHandler{
		AuthenticationHandler: handler,
		BrowserSession:        testBrowserSessionConfig(now),
	}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	bootstrapKey := uuid.New()
	preSession, csrfToken, err := identity.BrowserSession.issuePreSession(bootstrapKey)
	require.NoError(t, err)

	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(`{"username":"session-user","password":"correct horse battery staple"}`),
	)
	request.RemoteAddr = "192.0.2.25:43120"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-site")
	request.Header.Set("Idempotency-Key", idempotencyKey.String())
	request.Header.Set("User-Agent", "Judgify Test Browser/1.0")
	request.Header.Set(CSRFHeaderName, csrfToken)
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.PreSession.Name, Value: preSession})
	request.AddCookie(&http.Cookie{Name: identity.BrowserSession.CSRF.Name, Value: csrfToken})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NotNil(t, received)
	require.Equal(t, idempotencyKey, received.IdempotencyKey)
	require.Equal(t, "Judgify Test Browser/1.0", received.UserAgent)
	hasher := hmac.New(sha256.New, identity.BrowserSession.ipHashKey)
	_, _ = hasher.Write([]byte("192.0.2.25"))
	require.Equal(t, hasher.Sum(nil), received.IPHash)
	require.NotContains(t, response.Body.String(), "new-access-token")
	require.NotContains(t, response.Body.String(), "new-refresh-token")
	cookies := responseCookieMap(response)
	require.Equal(t, "new-access-token", cookies[identity.BrowserSession.Access.Name].Value)
}

func TestRefreshRouteFailsClosedOnAmbiguousBrowserAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	config := testBrowserSessionConfig(now)
	sessionID := uuid.New()
	csrf, err := config.issueSessionCSRF(sessionID, now.Add(24*time.Hour))
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(*http.Request)
		status int
	}{
		{
			name: "missing origin",
			mutate: func(request *http.Request) {
				request.Header.Del("Origin")
			},
			status: http.StatusForbidden,
		},
		{
			name: "untrusted origin",
			mutate: func(request *http.Request) {
				request.Header.Set("Origin", "https://evil.example")
			},
			status: http.StatusForbidden,
		},
		{
			name: "cross site fetch metadata",
			mutate: func(request *http.Request) {
				request.Header.Set("Sec-Fetch-Site", "cross-site")
			},
			status: http.StatusForbidden,
		},
		{
			name: "legacy bearer mixed with cookie",
			mutate: func(request *http.Request) {
				request.Header.Set("Authorization", "Bearer legacy-refresh")
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "missing idempotency key",
			mutate: func(request *http.Request) {
				request.Header.Del("Idempotency-Key")
			},
			status: http.StatusBadRequest,
		},
		{
			name: "duplicate idempotency key",
			mutate: func(request *http.Request) {
				request.Header.Add("Idempotency-Key", uuid.NewString())
			},
			status: http.StatusBadRequest,
		},
		{
			name: "duplicate refresh cookie",
			mutate: func(request *http.Request) {
				request.AddCookie(&http.Cookie{Name: config.Refresh.Name, Value: "duplicate"})
			},
			status: http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			router := gin.New()
			identity := &IdentityHandler{
				AuthenticationHandler: authenticationHandlerStub{refresh: func(
					context.Context,
					*dto.RefreshTokenRequest,
				) (*dto.RefreshTokenResponse, error) {
					called = true
					return nil, nil
				}},
				BrowserSession: config,
			}
			identity.RegisterRefresh(router.Group(""))

			request := refreshRequest(uuid.New())
			request.Header.Set(CSRFHeaderName, csrf)
			request.AddCookie(&http.Cookie{Name: config.CSRF.Name, Value: csrf})
			request.AddCookie(&http.Cookie{Name: config.Refresh.Name, Value: "presented-refresh-token"})
			test.mutate(request)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, test.status, response.Code, response.Body.String())
			require.False(t, called)
			require.NotContains(t, response.Body.String(), "presented-refresh-token")
		})
	}
}

func TestPasswordRecoveryRoutesNeverEchoSecretsAndDisableCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	rawToken := "jpr1." + strings.Repeat("A", 43)
	var receivedToken string
	handler := authenticationHandlerStub{
		forgot: func(_ context.Context, request *dto.ForgotPasswordRequest) (*dto.ForgotPasswordResponse, error) {
			require.Equal(t, "private-user", request.Username)
			require.Len(t, request.IPHash, 32)
			return &dto.ForgotPasswordResponse{
				Status: dto.ForgotPasswordStatusAccepted,
			}, nil
		},
		reset: func(_ context.Context, request *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
			receivedToken = request.Token
			return &dto.ResetPasswordResponse{Success: true}, nil
		},
	}
	identity := &IdentityHandler{AuthenticationHandler: handler, BrowserSession: testBrowserSessionConfig(now)}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	forgotBootstrapKey := uuid.New()
	preSession, csrfToken, err := identity.BrowserSession.issuePreSession(forgotBootstrapKey)
	require.NoError(t, err)

	forgotRequest := httptest.NewRequest(
		http.MethodPost,
		"/auth/forgot-password",
		strings.NewReader(`{"username":"private-user"}`),
	)
	forgotRequest.RemoteAddr = "192.0.2.1:12345"
	forgotRequest.Header.Set("Content-Type", "application/json")
	forgotRequest.Header.Set("Origin", "https://app.example")
	forgotRequest.Header.Set("Sec-Fetch-Site", "same-site")
	forgotRequest.Header.Set("Idempotency-Key", forgotBootstrapKey.String())
	forgotRequest.Header.Set(CSRFHeaderName, csrfToken)
	forgotRequest.AddCookie(&http.Cookie{Name: identity.BrowserSession.PreSession.Name, Value: preSession})
	forgotRequest.AddCookie(&http.Cookie{Name: identity.BrowserSession.CSRF.Name, Value: csrfToken})
	forgotResponse := httptest.NewRecorder()
	router.ServeHTTP(forgotResponse, forgotRequest)
	require.Equal(t, http.StatusAccepted, forgotResponse.Code, forgotResponse.Body.String())
	require.Contains(t, forgotResponse.Body.String(), `"status":"accepted"`)
	require.NotContains(t, forgotResponse.Body.String(), "private-user")
	require.Equal(t, "no-store", forgotResponse.Header().Get("Cache-Control"))

	resetRequest := httptest.NewRequest(
		http.MethodPost,
		"/auth/reset-password",
		strings.NewReader(`{"token":"`+rawToken+`","new_password":"new-password-123"}`),
	)
	resetRequest.Header.Set("Content-Type", "application/json")
	resetResponse := httptest.NewRecorder()
	router.ServeHTTP(resetResponse, resetRequest)
	require.Equal(t, http.StatusForbidden, resetResponse.Code, resetResponse.Body.String())
	require.Empty(t, receivedToken)

	resetBootstrapKey := uuid.New()
	resetPreSession, resetCSRF, err := identity.BrowserSession.issuePreSession(resetBootstrapKey)
	require.NoError(t, err)

	resetRequest = httptest.NewRequest(
		http.MethodPost,
		"/auth/reset-password",
		strings.NewReader(`{"token":"`+rawToken+`","new_password":"new-password-123"}`),
	)
	resetRequest.Header.Set("Content-Type", "application/json")
	resetRequest.Header.Set("Origin", "https://app.example")
	resetRequest.Header.Set("Sec-Fetch-Site", "same-site")
	resetRequest.Header.Set("Idempotency-Key", resetBootstrapKey.String())
	resetRequest.Header.Set(CSRFHeaderName, resetCSRF)
	resetRequest.AddCookie(&http.Cookie{Name: identity.BrowserSession.PreSession.Name, Value: resetPreSession})
	resetRequest.AddCookie(&http.Cookie{Name: identity.BrowserSession.CSRF.Name, Value: resetCSRF})
	resetResponse = httptest.NewRecorder()
	router.ServeHTTP(resetResponse, resetRequest)
	require.Equal(t, http.StatusOK, resetResponse.Code, resetResponse.Body.String())
	require.Equal(t, rawToken, receivedToken)
	require.NotContains(t, resetResponse.Body.String(), rawToken)
	require.Equal(t, "no-store", resetResponse.Header().Get("Cache-Control"))
}

func refreshRequest(idempotencyKey uuid.UUID) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(""))
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-site")
	request.Header.Set("Idempotency-Key", idempotencyKey.String())
	return request
}
