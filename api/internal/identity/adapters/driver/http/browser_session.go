package http

import (
	"bytes"
	"context"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/internal/authctx"
)

const (
	// CSRFHeaderName is shared with the outer CORS policy so browser mutation
	// preflights and the verifier cannot drift onto different header names.
	CSRFHeaderName               = "X-CSRF-Token"
	preSessionTokenPurpose       = "pre_session"
	preSessionCSRFTokenPurpose   = "pre_session_csrf"
	sessionCSRFTokenPurpose      = "session_csrf"
	browserTokenVersion          = 1
	maximumBrowserCookieBytes    = 16 << 10
	maximumBrowserSessionTTL     = 90 * 24 * time.Hour
	maximumPreSessionTTL         = 24 * time.Hour
	maximumBrowserTokenClockSkew = time.Minute
)

// BrowserCookiePolicy is the complete deployment-owned portion of one
// host-only browser cookie. Domain is deliberately absent: browser authority
// can never be widened to sibling hosts through configuration.
type BrowserCookiePolicy struct {
	Name     string
	Path     string
	Secure   bool
	SameSite nethttp.SameSite
}

// BrowserSessionConfig owns browser transport only. Core authentication
// services continue returning transport-neutral credentials for internal and
// non-browser adapters.
type BrowserSessionConfig struct {
	Access                  BrowserCookiePolicy
	Refresh                 BrowserCookiePolicy
	CSRF                    BrowserCookiePolicy
	PreSession              BrowserCookiePolicy
	OAuthLoginTicket        BrowserCookiePolicy
	OAuthLinkTicket         BrowserCookiePolicy
	PreSessionTTL           time.Duration
	OAuthCompletionURL      string
	AllowedOrigins          []string
	MaximumCredentialBytes  int
	MaximumUserAgentBytes   int
	ipHashKey               []byte
	signingKey              []byte
	preSessionDerivationKey []byte
	clock                   func() time.Time
}

type BrowserSessionOptions struct {
	Access                 BrowserCookiePolicy
	Refresh                BrowserCookiePolicy
	CSRF                   BrowserCookiePolicy
	PreSession             BrowserCookiePolicy
	OAuthLoginTicket       BrowserCookiePolicy
	OAuthLinkTicket        BrowserCookiePolicy
	PreSessionTTL          time.Duration
	OAuthCompletionURL     string
	AllowedOrigins         []string
	MaximumCredentialBytes int
	MaximumUserAgentBytes  int
	Secret                 []byte `json:"-"`
	Clock                  func() time.Time
}

// NewBrowserSessionConfig derives domain-separated keys so client metadata,
// signed browser state, and deterministic retry identity never share a MAC
// namespace.
func NewBrowserSessionConfig(options BrowserSessionOptions) (BrowserSessionConfig, error) {
	if len(options.Secret) < 32 || len(options.Secret) > 4096 ||
		strings.Contains(string(options.Secret), "${") {
		return BrowserSessionConfig{}, errors.New("identity HTTP: invalid browser-session protection key")
	}
	ipHashKey, err := deriveBrowserKey(options.Secret, "judgify/identity/client-ip-hmac/v1")
	if err != nil {
		return BrowserSessionConfig{}, err
	}
	signingKey, err := deriveBrowserKey(options.Secret, "judgify/identity/browser-state-hmac/v1")
	if err != nil {
		return BrowserSessionConfig{}, err
	}
	preSessionKey, err := deriveBrowserKey(options.Secret, "judgify/identity/pre-session-identity/v1")
	if err != nil {
		return BrowserSessionConfig{}, err
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	config := BrowserSessionConfig{
		Access: options.Access, Refresh: options.Refresh,
		CSRF: options.CSRF, PreSession: options.PreSession,
		OAuthLoginTicket:        options.OAuthLoginTicket,
		OAuthLinkTicket:         options.OAuthLinkTicket,
		PreSessionTTL:           options.PreSessionTTL,
		OAuthCompletionURL:      strings.TrimSpace(options.OAuthCompletionURL),
		AllowedOrigins:          append([]string(nil), options.AllowedOrigins...),
		MaximumCredentialBytes:  options.MaximumCredentialBytes,
		MaximumUserAgentBytes:   options.MaximumUserAgentBytes,
		ipHashKey:               append([]byte(nil), ipHashKey...),
		signingKey:              append([]byte(nil), signingKey...),
		preSessionDerivationKey: append([]byte(nil), preSessionKey...),
		clock:                   clock,
	}
	if err := config.Validate(); err != nil {
		return BrowserSessionConfig{}, err
	}
	return config, nil
}

func deriveBrowserKey(secret []byte, label string) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, secret, nil, label, sha256.Size)
	if err != nil {
		return nil, errors.New("identity HTTP: derive browser-session protection key")
	}
	return key, nil
}

func (config BrowserSessionConfig) Validate() error {
	policies := []BrowserCookiePolicy{
		config.Access,
		config.Refresh,
		config.CSRF,
		config.PreSession,
		config.OAuthLoginTicket,
		config.OAuthLinkTicket,
	}
	names := make([]string, 0, len(policies))
	for _, policy := range policies {
		if err := validateBrowserCookiePolicy(policy); err != nil ||
			slices.Contains(names, policy.Name) {
			return errors.New("identity HTTP: invalid browser cookie configuration")
		}
		names = append(names, policy.Name)
	}
	if config.Access.Path != "/" || config.CSRF.Path != "/" ||
		config.PreSession.Path != "/" ||
		!validOAuthTicketCookiePath(config.OAuthLoginTicket.Path, "/auth/oauth/login") ||
		!validOAuthTicketCookiePath(config.OAuthLinkTicket.Path, "/auth/oauth/link") ||
		config.Refresh.SameSite != nethttp.SameSiteStrictMode ||
		config.CSRF.SameSite != nethttp.SameSiteStrictMode ||
		config.PreSession.SameSite != nethttp.SameSiteStrictMode ||
		config.OAuthLoginTicket.SameSite != nethttp.SameSiteStrictMode ||
		config.OAuthLinkTicket.SameSite != nethttp.SameSiteStrictMode ||
		(config.Access.SameSite != nethttp.SameSiteLaxMode &&
			config.Access.SameSite != nethttp.SameSiteStrictMode) {
		return errors.New("identity HTTP: unsafe browser cookie policy")
	}
	if config.PreSessionTTL <= 0 || config.PreSessionTTL > maximumPreSessionTTL ||
		config.MaximumCredentialBytes < 1 ||
		config.MaximumCredentialBytes > maximumBrowserCookieBytes ||
		config.MaximumUserAgentBytes < 1 || config.MaximumUserAgentBytes > 512 ||
		len(config.AllowedOrigins) == 0 || len(config.AllowedOrigins) > 32 ||
		len(config.ipHashKey) != sha256.Size ||
		len(config.signingKey) != sha256.Size ||
		len(config.preSessionDerivationKey) != sha256.Size ||
		config.clock == nil {
		return errors.New("identity HTTP: invalid browser-session configuration")
	}
	normalized := make([]string, 0, len(config.AllowedOrigins))
	for _, raw := range config.AllowedOrigins {
		origin, err := normalizeBrowserOrigin(raw)
		if err != nil || slices.Contains(normalized, origin) {
			return errors.New("identity HTTP: invalid browser origin policy")
		}
		normalized = append(normalized, origin)
	}
	completion, err := url.Parse(config.OAuthCompletionURL)
	if err != nil || completion.Scheme == "" || completion.Host == "" ||
		completion.User != nil || completion.RawQuery != "" || completion.Fragment != "" ||
		completion.Path != "/auth/oauth/complete" || completion.RawPath != "" {
		return errors.New("identity HTTP: invalid oauth completion URL")
	}
	completionOrigin, err := normalizeBrowserOrigin(completion.Scheme + "://" + completion.Host)
	if err != nil || !slices.Contains(normalized, completionOrigin) {
		return errors.New("identity HTTP: oauth completion origin is not trusted")
	}
	return nil
}

func validOAuthTicketCookiePath(value, suffix string) bool {
	return strings.HasPrefix(value, "/") && strings.HasSuffix(value, suffix) &&
		!strings.Contains(value, "//") && !strings.Contains(value, "..") &&
		len(value) <= 256
}

func validateBrowserCookiePolicy(policy BrowserCookiePolicy) error {
	if policy.Name == "" || len(policy.Name) > 128 ||
		policy.Path == "" || policy.Path[0] != '/' ||
		(policy.SameSite != nethttp.SameSiteStrictMode &&
			policy.SameSite != nethttp.SameSiteLaxMode) {
		return errors.New("invalid browser cookie policy")
	}
	if strings.HasPrefix(policy.Name, "__Host-") &&
		(!policy.Secure || policy.Path != "/") ||
		strings.HasPrefix(policy.Name, "__Secure-") && !policy.Secure {
		return errors.New("invalid browser cookie prefix policy")
	}
	for _, character := range policy.Name {
		lower := character >= 'a' && character <= 'z'
		upper := character >= 'A' && character <= 'Z'
		digit := character >= '0' && character <= '9'
		special := strings.ContainsRune("!#$%&'*+-.^_`|~", character)
		if !lower && !upper && !digit && !special {
			return errors.New("invalid browser cookie name")
		}
	}
	return nil
}

func (config BrowserSessionConfig) allowsOrigin(origin string) bool {
	for _, raw := range config.AllowedOrigins {
		normalized, err := normalizeBrowserOrigin(raw)
		if err == nil && normalized == origin {
			return true
		}
	}
	return false
}

type browserStateClaims struct {
	Version   int    `json:"v"`
	Purpose   string `json:"p"`
	Subject   string `json:"s"`
	Nonce     string `json:"n"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func (config BrowserSessionConfig) signClaims(claims browserStateClaims) (string, error) {
	if config.Validate() != nil || claims.Version != browserTokenVersion ||
		claims.Purpose == "" || claims.Subject == "" || claims.Nonce == "" ||
		claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return "", errors.New("invalid browser state")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", errors.New("encode browser state")
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, config.signingKey)
	_, _ = mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + signature, nil
}

func (config BrowserSessionConfig) parseClaims(
	raw string,
	expectedPurpose string,
) (browserStateClaims, error) {
	if config.Validate() != nil || raw == "" ||
		len(raw) > maximumBrowserCookieBytes ||
		strings.TrimSpace(raw) != raw ||
		strings.Count(raw, ".") != 1 {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	parts := strings.SplitN(raw, ".", 2)
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	mac := hmac.New(sha256.New, config.signingKey)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) == 0 || len(payload) > maximumBrowserCookieBytes {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var claims browserStateClaims
	if err := decoder.Decode(&claims); err != nil {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	now := config.clock().UTC()
	issuedAt := time.Unix(claims.IssuedAt, 0).UTC()
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if claims.Version != browserTokenVersion ||
		claims.Purpose != expectedPurpose ||
		claims.Subject == "" || claims.Nonce == "" ||
		issuedAt.After(now.Add(maximumBrowserTokenClockSkew)) ||
		!expiresAt.After(now) ||
		!expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > maximumBrowserSessionTTL {
		return browserStateClaims{}, errors.New("invalid browser state")
	}
	return claims, nil
}

func (config BrowserSessionConfig) deterministicPreSession(
	idempotencyKey uuid.UUID,
) (uuid.UUID, string, error) {
	if idempotencyKey == uuid.Nil || len(config.preSessionDerivationKey) != sha256.Size {
		return uuid.Nil, "", errors.New("invalid pre-session identity")
	}
	identifierMAC := hmac.New(sha256.New, config.preSessionDerivationKey)
	_, _ = identifierMAC.Write([]byte("id|" + idempotencyKey.String()))
	sum := identifierMAC.Sum(nil)
	var identifier uuid.UUID
	copy(identifier[:], sum[:len(identifier)])
	identifier[6] = identifier[6]&0x0f | 0x40
	identifier[8] = identifier[8]&0x3f | 0x80

	nonceMAC := hmac.New(sha256.New, config.preSessionDerivationKey)
	_, _ = nonceMAC.Write([]byte("nonce|" + idempotencyKey.String()))
	nonce := base64.RawURLEncoding.EncodeToString(nonceMAC.Sum(nil))
	return identifier, nonce, nil
}

func (config BrowserSessionConfig) issuePreSession(
	idempotencyKey uuid.UUID,
) (string, string, error) {
	preSessionID, nonce, err := config.deterministicPreSession(idempotencyKey)
	if err != nil {
		return "", "", err
	}
	now := config.clock().UTC()
	expiresAt := now.Add(config.PreSessionTTL)
	base := browserStateClaims{
		Version: browserTokenVersion, Subject: preSessionID.String(),
		Nonce: nonce, IssuedAt: now.Unix(), ExpiresAt: expiresAt.Unix(),
	}
	preClaims := base
	preClaims.Purpose = preSessionTokenPurpose
	preSession, err := config.signClaims(preClaims)
	if err != nil {
		return "", "", err
	}
	csrfClaims := base
	csrfClaims.Purpose = preSessionCSRFTokenPurpose
	csrf, err := config.signClaims(csrfClaims)
	if err != nil {
		return "", "", err
	}
	return preSession, csrf, nil
}

func (config BrowserSessionConfig) issueSessionCSRF(
	sessionID uuid.UUID,
	expiresAt time.Time,
) (string, error) {
	now := config.clock().UTC()
	expiresAt = expiresAt.UTC()
	if sessionID == uuid.Nil || !expiresAt.After(now) ||
		expiresAt.Sub(now) > maximumBrowserSessionTTL {
		return "", errors.New("invalid session CSRF boundary")
	}
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", errors.New("generate session CSRF nonce")
	}
	return config.signClaims(browserStateClaims{
		Version: browserTokenVersion, Purpose: sessionCSRFTokenPurpose,
		Subject:  sessionID.String(),
		Nonce:    base64.RawURLEncoding.EncodeToString(nonceBytes),
		IssuedAt: now.Unix(), ExpiresAt: expiresAt.Unix(),
	})
}

func singularCookie(request *nethttp.Request, name string) (*nethttp.Cookie, error) {
	if request == nil || name == "" {
		return nil, errors.New("invalid browser cookie")
	}
	cookies := request.CookiesNamed(name)
	if len(cookies) != 1 || cookies[0].Value == "" {
		return nil, errors.New("invalid browser cookie")
	}
	return cookies[0], nil
}

func singularHeader(request *nethttp.Request, name string) (string, error) {
	if request == nil {
		return "", errors.New("invalid browser header")
	}
	values := request.Header.Values(name)
	if len(values) != 1 || values[0] == "" ||
		len(values[0]) > maximumBrowserCookieBytes ||
		strings.TrimSpace(values[0]) != values[0] {
		return "", errors.New("invalid browser header")
	}
	return values[0], nil
}

func (config BrowserSessionConfig) existingPreSession(
	request *nethttp.Request,
) (uuid.UUID, bool) {
	preCookie, err := singularCookie(request, config.PreSession.Name)
	if err != nil {
		return uuid.Nil, false
	}
	csrfCookie, err := singularCookie(request, config.CSRF.Name)
	if err != nil {
		return uuid.Nil, false
	}
	preClaims, err := config.parseClaims(preCookie.Value, preSessionTokenPurpose)
	if err != nil {
		return uuid.Nil, false
	}
	csrfClaims, err := config.parseClaims(csrfCookie.Value, preSessionCSRFTokenPurpose)
	if err != nil || !matchingBrowserState(preClaims, csrfClaims) {
		return uuid.Nil, false
	}
	identifier, err := uuid.Parse(preClaims.Subject)
	if err != nil || identifier == uuid.Nil || identifier.String() != preClaims.Subject {
		return uuid.Nil, false
	}
	return identifier, true
}

func matchingBrowserState(left, right browserStateClaims) bool {
	return hmac.Equal([]byte(left.Subject), []byte(right.Subject)) &&
		hmac.Equal([]byte(left.Nonce), []byte(right.Nonce)) &&
		left.IssuedAt == right.IssuedAt &&
		left.ExpiresAt == right.ExpiresAt
}

func (config BrowserSessionConfig) verifyPreSessionRequest(
	request *nethttp.Request,
) (uuid.UUID, error) {
	header, err := singularHeader(request, CSRFHeaderName)
	if err != nil {
		return uuid.Nil, err
	}
	csrfCookie, err := singularCookie(request, config.CSRF.Name)
	if err != nil || !hmac.Equal([]byte(header), []byte(csrfCookie.Value)) {
		return uuid.Nil, errors.New("invalid pre-session CSRF")
	}
	return config.requireExistingPreSession(request)
}

func (config BrowserSessionConfig) requireExistingPreSession(
	request *nethttp.Request,
) (uuid.UUID, error) {
	identifier, ok := config.existingPreSession(request)
	if !ok {
		return uuid.Nil, errors.New("invalid pre-session CSRF")
	}
	return identifier, nil
}

func (config BrowserSessionConfig) verifySessionCSRF(
	request *nethttp.Request,
	expectedSessionID uuid.UUID,
) error {
	if expectedSessionID == uuid.Nil {
		return errors.New("invalid session CSRF authority")
	}
	header, err := singularHeader(request, CSRFHeaderName)
	if err != nil {
		return err
	}
	cookie, err := singularCookie(request, config.CSRF.Name)
	if err != nil || !hmac.Equal([]byte(header), []byte(cookie.Value)) {
		return errors.New("invalid session CSRF")
	}
	claims, err := config.parseClaims(cookie.Value, sessionCSRFTokenPurpose)
	if err != nil || claims.Subject != expectedSessionID.String() {
		return errors.New("invalid session CSRF")
	}
	return nil
}

func (config BrowserSessionConfig) sessionIDFromCSRF(
	request *nethttp.Request,
) (uuid.UUID, error) {
	header, err := singularHeader(request, CSRFHeaderName)
	if err != nil {
		return uuid.Nil, err
	}
	cookie, err := singularCookie(request, config.CSRF.Name)
	if err != nil || !hmac.Equal([]byte(header), []byte(cookie.Value)) {
		return uuid.Nil, errors.New("invalid session CSRF")
	}
	claims, err := config.parseClaims(cookie.Value, sessionCSRFTokenPurpose)
	if err != nil {
		return uuid.Nil, err
	}
	sessionID, err := uuid.Parse(claims.Subject)
	if err != nil || sessionID == uuid.Nil || sessionID.String() != claims.Subject {
		return uuid.Nil, errors.New("invalid session CSRF")
	}
	return sessionID, nil
}

func (config BrowserSessionConfig) makeCookie(
	policy BrowserCookiePolicy,
	value string,
	expiresAt time.Time,
	httpOnly bool,
) (*nethttp.Cookie, error) {
	now := config.clock().UTC()
	expiresAt = expiresAt.UTC()
	if !validBrowserCookieValue(value, config.MaximumCredentialBytes) ||
		!expiresAt.After(now) ||
		expiresAt.Sub(now) > maximumBrowserSessionTTL {
		return nil, errors.New("invalid browser cookie value")
	}
	remaining := expiresAt.Sub(now)
	return &nethttp.Cookie{
		Name: policy.Name, Value: value, Path: policy.Path,
		Expires:  expiresAt,
		MaxAge:   max(1, int((remaining+time.Second-1)/time.Second)),
		HttpOnly: httpOnly, Secure: policy.Secure, SameSite: policy.SameSite,
	}, nil
}

func validBrowserCookieValue(value string, maximum int) bool {
	if value == "" || maximum < 1 || len(value) > maximum ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character <= 0x20 || character == 0x7f ||
			character == ';' || character == ',' {
			return false
		}
	}
	return true
}

func expiredCookie(policy BrowserCookiePolicy, httpOnly bool) *nethttp.Cookie {
	return &nethttp.Cookie{
		Name: policy.Name, Path: policy.Path,
		Expires: time.Unix(1, 0).UTC(), MaxAge: -1,
		HttpOnly: httpOnly, Secure: policy.Secure, SameSite: policy.SameSite,
	}
}

func (h *IdentityHandler) writeSessionCookies(
	c *gin.Context,
	sessionID uuid.UUID,
	accessToken string,
	accessExpiresAt time.Time,
	refreshToken string,
	refreshExpiresAt time.Time,
) error {
	if h == nil || c == nil || h.BrowserSession.Validate() != nil ||
		sessionID == uuid.Nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil)
	}
	accessCookie, err := h.BrowserSession.makeCookie(
		h.BrowserSession.Access,
		accessToken,
		accessExpiresAt,
		true,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	refreshCookie, err := h.BrowserSession.makeCookie(
		h.BrowserSession.Refresh,
		refreshToken,
		refreshExpiresAt,
		true,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	csrf, err := h.BrowserSession.issueSessionCSRF(sessionID, refreshExpiresAt)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	csrfCookie, err := h.BrowserSession.makeCookie(
		h.BrowserSession.CSRF,
		csrf,
		refreshExpiresAt,
		false,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	nethttp.SetCookie(c.Writer, accessCookie)
	nethttp.SetCookie(c.Writer, refreshCookie)
	nethttp.SetCookie(c.Writer, csrfCookie)
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.PreSession, true))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLoginTicket, true))
	return nil
}

func (h *IdentityHandler) writePreSessionCookies(
	c *gin.Context,
	idempotencyKey uuid.UUID,
) error {
	preSession, csrf, err := h.BrowserSession.issuePreSession(idempotencyKey)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	expiresAt := h.BrowserSession.clock().UTC().Add(h.BrowserSession.PreSessionTTL)
	preCookie, err := h.BrowserSession.makeCookie(
		h.BrowserSession.PreSession,
		preSession,
		expiresAt,
		true,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	csrfCookie, err := h.BrowserSession.makeCookie(
		h.BrowserSession.CSRF,
		csrf,
		expiresAt,
		false,
	)
	if err != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err)
	}
	nethttp.SetCookie(c.Writer, preCookie)
	nethttp.SetCookie(c.Writer, csrfCookie)
	return nil
}

// bootstrapPreSession establishes the anonymous, CSRF-bound identity used by
// credential-issuing browser commands. It is deliberately idempotent: an
// already valid pair is never rotated by a retry with a new command key.
func (h *IdentityHandler) bootstrapPreSession(c *gin.Context) {
	if h == nil || c == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	if _, ok := h.BrowserSession.existingPreSession(c.Request); ok {
		noStore(c)
		c.Status(nethttp.StatusNoContent)
		return
	}
	if err := rejectBrowserCredentialAmbiguity(c.Request, h.BrowserSession); err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeBadRequest, "ambiguous browser credentials", err))
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if err := h.writePreSessionCookies(c, idempotencyKey); err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	c.Status(nethttp.StatusNoContent)
}

func (h *IdentityHandler) requirePreSessionMutation(c *gin.Context) (uuid.UUID, error) {
	if h == nil || c == nil || h.BrowserSession.Validate() != nil {
		return uuid.Nil, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil)
	}
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		return uuid.Nil, err
	}
	preSessionID, err := h.BrowserSession.verifyPreSessionRequest(c.Request)
	if err != nil {
		return uuid.Nil, apperr.New(apperr.CodeForbidden, "csrf verification failed", err)
	}
	return preSessionID, nil
}

func (h *IdentityHandler) clearSessionCookies(c *gin.Context) {
	if h == nil || c == nil {
		return
	}
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.Access, true))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.Refresh, true))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.CSRF, false))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.PreSession, true))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLoginTicket, true))
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLinkTicket, true))
}

type browserCSRFVerifier struct {
	config BrowserSessionConfig
}

func (verifier browserCSRFVerifier) Verify(
	ctx context.Context,
	request *nethttp.Request,
) error {
	source, ok := middlewares.CredentialSourceFromContext(ctx)
	if !ok {
		return errors.New("missing credential source")
	}
	if source == middlewares.CredentialBearer {
		return nil
	}
	if source != middlewares.CredentialCookie {
		return errors.New("unsupported credential source")
	}
	if err := verifier.config.requireTrustedBrowserMutation(request); err != nil {
		return err
	}
	sessionID, ok := authctx.SessionID(ctx)
	if !ok {
		return errors.New("missing authenticated session")
	}
	return verifier.config.verifySessionCSRF(request, sessionID)
}

func (h *IdentityHandler) ProtectedCSRFMiddleware() (gin.HandlerFunc, error) {
	if h == nil || h.BrowserSession.Validate() != nil {
		return nil, errors.New("identity HTTP: invalid browser-session configuration")
	}
	return middlewares.CSRF(browserCSRFVerifier{config: h.BrowserSession})
}

func (config BrowserSessionConfig) requireTrustedBrowserMutation(
	request *nethttp.Request,
) error {
	if config.Validate() != nil || request == nil {
		return errors.New("authentication transport unavailable")
	}
	origins := request.Header.Values("Origin")
	if len(origins) != 1 {
		return errors.New("untrusted browser origin")
	}
	origin, err := normalizeBrowserOrigin(origins[0])
	if err != nil || !config.allowsOrigin(origin) {
		return errors.New("untrusted browser origin")
	}
	switch fetchSite := strings.ToLower(strings.TrimSpace(
		request.Header.Get("Sec-Fetch-Site"),
	)); fetchSite {
	case "", "same-origin", "same-site", "none":
		return nil
	default:
		return errors.New("untrusted browser request context")
	}
}

func rejectBrowserCredentialAmbiguity(
	request *nethttp.Request,
	config BrowserSessionConfig,
) error {
	if request == nil {
		return errors.New("invalid browser request")
	}
	if len(request.Header.Values("Authorization")) != 0 ||
		len(request.CookiesNamed(config.Access.Name)) != 0 ||
		len(request.CookiesNamed(config.Refresh.Name)) != 0 {
		return errors.New("ambiguous browser credentials")
	}
	return nil
}
