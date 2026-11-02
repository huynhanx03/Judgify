package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/security/authentication"
	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

func TestPreSessionBootstrapIsStableAndLoginReturnsOnlyCookieAuthority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 28, 9, 30, 0, 0, time.UTC)
	config := testBrowserSessionConfig(now)
	sessionID := uuid.MustParse("019f6abb-8dd5-7581-8449-2b9ad77873e5")
	var loginCommand *dto.LoginRequest
	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{
			login: func(_ context.Context, request *dto.LoginRequest) (*dto.LoginResponse, error) {
				copied := *request
				loginCommand = &copied
				return &dto.LoginResponse{
					SessionID:             sessionID,
					AccessToken:           "browser-access-secret",
					AccessTokenExpiresAt:  now.Add(15 * time.Minute),
					RefreshToken:          "browser-refresh-secret",
					RefreshTokenExpiresAt: now.Add(24 * time.Hour),
				}, nil
			},
		},
		BrowserSession: config,
	}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	bootstrapKey := uuid.MustParse("019f6abb-8dd5-7581-8449-2b9ad77873e6")
	firstBootstrap := browserMutationRequest(nethttp.MethodPost, "/auth/pre-session", bootstrapKey, "")
	firstRecorder := httptest.NewRecorder()
	router.ServeHTTP(firstRecorder, firstBootstrap)
	require.Equal(t, nethttp.StatusNoContent, firstRecorder.Code, firstRecorder.Body.String())
	firstCookies := responseCookieMap(firstRecorder)
	require.NotEmpty(t, firstCookies[config.PreSession.Name])
	require.NotEmpty(t, firstCookies[config.CSRF.Name])
	require.True(t, firstCookies[config.PreSession.Name].HttpOnly)
	require.False(t, firstCookies[config.CSRF.Name].HttpOnly)
	require.Empty(t, firstCookies[config.PreSession.Name].Domain)
	require.Empty(t, firstCookies[config.CSRF.Name].Domain)

	secondBootstrap := browserMutationRequest(nethttp.MethodPost, "/auth/pre-session", uuid.New(), "")
	secondBootstrap.AddCookie(firstCookies[config.PreSession.Name])
	secondBootstrap.AddCookie(firstCookies[config.CSRF.Name])
	secondRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondRecorder, secondBootstrap)
	require.Equal(t, nethttp.StatusNoContent, secondRecorder.Code, secondRecorder.Body.String())
	require.Empty(t, secondRecorder.Header().Values("Set-Cookie"), "valid bootstrap must not rotate")

	loginKey := uuid.MustParse("019f6abb-8dd5-7581-8449-2b9ad77873e7")
	loginRequest := browserMutationRequest(
		nethttp.MethodPost,
		"/auth/login",
		loginKey,
		`{"username":"session-user","password":"correct horse battery staple"}`,
	)
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set(CSRFHeaderName, firstCookies[config.CSRF.Name].Value)
	loginRequest.AddCookie(firstCookies[config.PreSession.Name])
	loginRequest.AddCookie(firstCookies[config.CSRF.Name])
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, loginRequest)

	require.Equal(t, nethttp.StatusOK, loginRecorder.Code, loginRecorder.Body.String())
	require.NotNil(t, loginCommand)
	require.NotEqual(t, uuid.Nil, loginCommand.PreSessionID)
	require.NotEqual(t, loginKey, loginCommand.PreSessionID)
	require.NotContains(t, loginRecorder.Body.String(), "browser-access-secret")
	require.NotContains(t, loginRecorder.Body.String(), "browser-refresh-secret")
	require.Contains(t, loginRecorder.Body.String(), `"success":true`)

	sessionCookies := responseCookieMap(loginRecorder)
	require.Equal(t, "browser-access-secret", sessionCookies[config.Access.Name].Value)
	require.Equal(t, "browser-refresh-secret", sessionCookies[config.Refresh.Name].Value)
	require.NotEmpty(t, sessionCookies[config.CSRF.Name].Value)
	require.True(t, sessionCookies[config.Access.Name].HttpOnly)
	require.True(t, sessionCookies[config.Refresh.Name].HttpOnly)
	require.False(t, sessionCookies[config.CSRF.Name].HttpOnly)
	require.Equal(t, -1, sessionCookies[config.PreSession.Name].MaxAge)
}

func TestLoginRejectsMissingOrMismatchedAnonymousCSRFBeforeCore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{
			login: func(context.Context, *dto.LoginRequest) (*dto.LoginResponse, error) {
				called = true
				return nil, nil
			},
		},
		BrowserSession: testBrowserSessionConfig(time.Now().UTC()),
	}
	router := gin.New()
	identity.RegisterPublic(router.Group(""))

	request := browserMutationRequest(
		nethttp.MethodPost,
		"/auth/login",
		uuid.New(),
		`{"username":"session-user","password":"correct horse battery staple"}`,
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusForbidden, recorder.Code, recorder.Body.String())
	require.False(t, called)
}

func TestRefreshRequiresSessionBoundCSRFAndNeverSerializesCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	config := testBrowserSessionConfig(now)
	sessionID := uuid.New()
	csrf, err := config.issueSessionCSRF(sessionID, now.Add(24*time.Hour))
	require.NoError(t, err)
	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{
			refresh: func(_ context.Context, request *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
				require.Equal(t, "presented-refresh", request.RefreshToken)
				return &dto.RefreshTokenResponse{
					SessionID:             sessionID,
					AccessToken:           "rotated-access",
					AccessTokenExpiresAt:  now.Add(15 * time.Minute),
					RefreshToken:          "rotated-refresh",
					RefreshTokenExpiresAt: now.Add(24 * time.Hour),
				}, nil
			},
		},
		BrowserSession: config,
	}
	router := gin.New()
	identity.RegisterRefresh(router.Group(""))

	request := browserMutationRequest(nethttp.MethodPost, "/auth/refresh", uuid.New(), "")
	request.Header.Set(CSRFHeaderName, csrf)
	request.AddCookie(&nethttp.Cookie{Name: config.Refresh.Name, Value: "presented-refresh"})
	request.AddCookie(&nethttp.Cookie{Name: config.CSRF.Name, Value: csrf})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusOK, recorder.Code, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "rotated-access")
	require.NotContains(t, recorder.Body.String(), "rotated-refresh")
	require.Contains(t, recorder.Body.String(), `"success":true`)
	cookies := responseCookieMap(recorder)
	require.Equal(t, "rotated-access", cookies[config.Access.Name].Value)
	require.Equal(t, "rotated-refresh", cookies[config.Refresh.Name].Value)
	require.NotEqual(t, csrf, cookies[config.CSRF.Name].Value)
}

func TestLogoutClearsAllBrowserAuthorityEvenWhenRevocationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	config := testBrowserSessionConfig(now)
	sessionID := uuid.New()

	identity := &IdentityHandler{
		AuthenticationHandler: authenticationHandlerStub{
			logout: func(context.Context) (*dto.LogoutResponse, error) {
				return nil, errors.New("database unavailable")
			},
		},
		BrowserSession: config,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		principal := authentication.Principal{
			Authenticated:      true,
			Subject:            sessionID.String(),
			UserID:             sessionID.String(),
			SessionID:          sessionID.String(),
			Username:           "test-user",
			TokenID:            uuid.New().String(),
			AuthenticatedAt:    now.Add(-time.Minute),
			SecurityVersion:    1,
			AuthenticatedUntil: now.Add(15 * time.Minute),
		}
		c.Request = c.Request.WithContext(
			authentication.WithPrincipal(c.Request.Context(), principal),
		)
		c.Next()
	})
	identity.RegisterProtected(router.Group(""), nil)

	request := browserMutationRequest(nethttp.MethodPost, "/auth/logout", uuid.New(), "")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	allBrowserAuthorityCookies := []BrowserCookiePolicy{
		config.Access,
		config.Refresh,
		config.CSRF,
		config.PreSession,
		config.OAuthLoginTicket,
		config.OAuthLinkTicket,
	}
	for _, policy := range allBrowserAuthorityCookies {
		cookie := responseCookieMap(recorder)[policy.Name]
		require.NotNil(t, cookie, "cookie %s must be expired on logout", policy.Name)
		require.Equal(t, -1, cookie.MaxAge, "cookie %s must be expired on logout", policy.Name)
	}
	require.Equal(t, nethttp.StatusInternalServerError, recorder.Code)
}

func TestSessionMeReturnsBoundedCanonicalProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()
	tokenID := uuid.New()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          sessionID.String(),
		Username:           "jerry",
		TokenID:            tokenID.String(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    2,
		AuthenticatedUntil: now.Add(14 * time.Minute),
	}
	identity := &IdentityHandler{
		AuthorizationHandler: sessionAuthorizationStub{
			me: &effectiveCapabilitiesResponse{
				Revision: 42,
				Capabilities: []capabilityGrantResponse{
					{Resource: "submission", Actions: []string{"inspect"}},
					{Resource: "contest", Actions: []string{"update"}},
				},
			},
		},
		UserHandler: sessionUserStub{
			profile: &profileAttrsResponse{
				ID: userID.String(), Username: "jerry", FirstName: "Jerry", LastName: "Nguyen",
			},
		},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authentication.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	})
	identity.RegisterProtected(router.Group(""), nil)

	request := httptest.NewRequest(nethttp.MethodGet, "/session/me", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"id":"`+userID.String()+`"`)
	require.Contains(t, recorder.Body.String(), `"id":"`+sessionID.String()+`"`)
	require.Contains(t, recorder.Body.String(), `"capabilities":["contest:update","submission:inspect"]`)
	require.Contains(t, recorder.Body.String(), `"authorization_revision":42`)
	require.NotContains(t, recorder.Body.String(), "roles")
	require.NotContains(t, recorder.Body.String(), "token")
}

type sessionAuthorizationStub struct {
	me *effectiveCapabilitiesResponse
}

func (stub sessionAuthorizationStub) Catalog(context.Context) (*authorizationCatalogResponse, error) {
	return nil, nil
}

func (stub sessionAuthorizationStub) Me(context.Context) (*effectiveCapabilitiesResponse, error) {
	return stub.me, nil
}

type sessionUserStub struct {
	UserHandler
	profile *profileAttrsResponse
}

func (stub sessionUserStub) GetProfile(context.Context) (*profileAttrsResponse, error) {
	return stub.profile, nil
}

func browserMutationRequest(method, target string, idempotencyKey uuid.UUID, body string) *nethttp.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = "192.0.2.25:43120"
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Idempotency-Key", idempotencyKey.String())
	request.Header.Set("User-Agent", "Judgify Test Browser/1.0")
	return request
}

func testBrowserSessionConfig(now time.Time) BrowserSessionConfig {
	config, err := NewBrowserSessionConfig(BrowserSessionOptions{
		Access:                 BrowserCookiePolicy{Name: "judgify_access", Path: "/", Secure: true, SameSite: nethttp.SameSiteLaxMode},
		Refresh:                BrowserCookiePolicy{Name: "judgify_refresh", Path: "/auth/refresh", Secure: true, SameSite: nethttp.SameSiteStrictMode},
		CSRF:                   BrowserCookiePolicy{Name: "judgify_csrf", Path: "/", Secure: true, SameSite: nethttp.SameSiteStrictMode},
		PreSession:             BrowserCookiePolicy{Name: "judgify_pre_session", Path: "/", Secure: true, SameSite: nethttp.SameSiteStrictMode},
		OAuthLoginTicket:       BrowserCookiePolicy{Name: "judgify_oauth_login", Path: "/auth/oauth/login", Secure: true, SameSite: nethttp.SameSiteStrictMode},
		OAuthLinkTicket:        BrowserCookiePolicy{Name: "judgify_oauth_link", Path: "/auth/oauth/link", Secure: true, SameSite: nethttp.SameSiteStrictMode},
		PreSessionTTL:          30 * time.Minute,
		OAuthCompletionURL:     "https://app.example/auth/oauth/complete",
		AllowedOrigins:         []string{"https://app.example"},
		MaximumCredentialBytes: 4096, MaximumUserAgentBytes: 512,
		Secret: []byte("0123456789abcdef0123456789abcdef"),
		Clock:  func() time.Time { return now },
	})
	if err != nil {
		panic(err)
	}
	return config
}

func responseCookieMap(recorder *httptest.ResponseRecorder) map[string]*nethttp.Cookie {
	result := make(map[string]*nethttp.Cookie)
	for _, cookie := range recorder.Result().Cookies() {
		result[cookie.Name] = cookie
	}
	return result
}
