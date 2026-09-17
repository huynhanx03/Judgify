package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"net"
	nethttp "net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
)

type successResponse struct {
	Success bool `json:"success"`
}

func (h *IdentityHandler) registerSession(c *gin.Context) {
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	req, err := commonrequest.ParseRequest[registerRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if err := h.applySessionCommandMetadata(c, &req.IdempotencyKey, &req.UserAgent, &req.IPHash); err != nil {
		identityHTTPFail(c, err)
		return
	}
	req.PreSessionID = preSessionID
	result, err := h.AuthenticationHandler.Register(c.Request.Context(), toRegisterRequest(req))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil || result.SessionID == uuid.Nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session issuance", nil))
		return
	}
	if err := h.writeSessionCookies(c, result.SessionID, result.AccessToken, result.AccessTokenExpiresAt, result.RefreshToken, result.RefreshTokenExpiresAt); err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, &successResponse{Success: result.Success})
}

func (h *IdentityHandler) loginSession(c *gin.Context) {
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	req, err := commonrequest.ParseRequest[loginRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if err := h.applySessionCommandMetadata(c, &req.IdempotencyKey, &req.UserAgent, &req.IPHash); err != nil {
		identityHTTPFail(c, err)
		return
	}
	req.PreSessionID = preSessionID
	result, err := h.AuthenticationHandler.Login(c.Request.Context(), toLoginRequest(req))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil || result.SessionID == uuid.Nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session issuance", nil))
		return
	}
	if err := h.writeSessionCookies(c, result.SessionID, result.AccessToken, result.AccessTokenExpiresAt, result.RefreshToken, result.RefreshTokenExpiresAt); err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, &successResponse{Success: true})
}

func (h *IdentityHandler) forgotPassword(c *gin.Context) {
	if _, err := h.requirePreSessionMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[forgotPasswordRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	request.IPHash, err = h.clientIPHash(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.AuthenticationHandler.ForgotPassword(c.Request.Context(), toForgotPasswordRequest(request))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.RespondStatus(c, nethttp.StatusAccepted, toForgotPasswordResponse(result))
}

func (h *IdentityHandler) resetPassword(c *gin.Context) {
	if _, err := h.requirePreSessionMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[resetPasswordRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.AuthenticationHandler.ResetPassword(c.Request.Context(), toResetPasswordRequest(request))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, toResetPasswordResponse(result))
}

// oauthStartSession creates provider state only after proving the same signed
// anonymous CSRF identity used by login and registration. The web application
// receives the provider navigation and completes the exchange through the
// same-origin, CSRF-protected callback command below.
func (h *IdentityHandler) oauthStartSession(c *gin.Context) {
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[oauthStartRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	command := toOAuthStartRequest(request)
	command.Purpose = entity.OAuthPurposeLogin
	command.PreSessionID = preSessionID
	result, err := h.AuthenticationHandler.OAuthStart(c.Request.Context(), command)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLoginTicket, true))
	noStore(c)
	response.Respond(c, toOAuthStartResponse(result))
}

func (h *IdentityHandler) oauthLinkStartSession(c *gin.Context) {
	if h == nil || h.AuthenticationHandler == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	request, err := commonrequest.ParseRequest[oauthStartRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	command := toOAuthStartRequest(request)
	command.Purpose = entity.OAuthPurposeLink
	result, err := h.AuthenticationHandler.OAuthLinkStart(c.Request.Context(), command)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLinkTicket, true))
	noStore(c)
	response.Respond(c, toOAuthStartResponse(result))
}

func (h *IdentityHandler) refreshSession(c *gin.Context) {
	if h == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	// Refresh has one authority only: the purpose-scoped HttpOnly cookie.
	// Rejecting bearer input prevents legacy and cookie credentials from being
	// accidentally combined during the cutover.
	if len(c.Request.Header.Values("Authorization")) != 0 {
		identityHTTPFail(c, apperr.New(apperr.CodeUnauthorized, "invalid refresh credential", nil))
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	sessionID, err := h.BrowserSession.sessionIDFromCSRF(c.Request)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeForbidden, "csrf verification failed", err))
		return
	}
	cookies := c.Request.CookiesNamed(h.BrowserSession.Refresh.Name)
	if len(cookies) != 1 || cookies[0].Value == "" {
		identityHTTPFail(c, apperr.New(apperr.CodeUnauthorized, "invalid refresh credential", nil))
		return
	}
	result, err := h.AuthenticationHandler.RefreshToken(c.Request.Context(), &dto.RefreshTokenRequest{
		RefreshToken:   cookies[0].Value,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		var applicationError *apperr.AppError
		if errors.As(err, &applicationError) && applicationError.Code == apperr.CodeUnauthorized {
			h.clearSessionCookies(c)
		}
		identityHTTPFail(c, err)
		return
	}
	if result == nil || result.SessionID == uuid.Nil || result.SessionID != sessionID {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session issuance", nil))
		return
	}
	if err := h.writeSessionCookies(c, result.SessionID, result.AccessToken, result.AccessTokenExpiresAt, result.RefreshToken, result.RefreshTokenExpiresAt); err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, &successResponse{Success: true})
}

func (h *IdentityHandler) logoutSession(c *gin.Context) {
	if h == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.AuthenticationHandler.Logout(c.Request.Context())
	h.clearSessionCookies(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, result)
}

// changePasswordSession mirrors the domain's revoke-all security boundary at
// the browser transport. Once the password commit succeeds, no stale access,
// refresh, CSRF, or pre-session cookie is left in the browser.
func (h *IdentityHandler) changePasswordSession(c *gin.Context) {
	if h == nil || h.AuthenticationHandler == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	request, err := commonrequest.ParseRequest[changePasswordRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.AuthenticationHandler.ChangePassword(c.Request.Context(), toChangePasswordRequest(request))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil || !result.Success {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid password change result", nil))
		return
	}
	h.clearSessionCookies(c)
	noStore(c)
	response.Respond(c, toChangePasswordResponse(result))
}

func (h *IdentityHandler) applySessionCommandMetadata(
	c *gin.Context,
	idempotencyKey *uuid.UUID,
	userAgent *string,
	ipHash *[]byte,
) error {
	parsed, err := parseIdempotencyHeader(c)
	if err != nil {
		return err
	}
	normalizedUserAgent := strings.TrimSpace(c.Request.UserAgent())
	if h == nil || h.BrowserSession.Validate() != nil ||
		!validTransportText(normalizedUserAgent, h.BrowserSession.MaximumUserAgentBytes) {
		return apperr.New(apperr.CodeBadRequest, "invalid user agent", nil)
	}
	hashedIP, err := h.clientIPHash(c)
	if err != nil {
		return err
	}
	*idempotencyKey = parsed
	*userAgent = normalizedUserAgent
	*ipHash = append((*ipHash)[:0], hashedIP...)
	return nil
}

func (h *IdentityHandler) clientIPHash(c *gin.Context) ([]byte, error) {
	if h == nil || c == nil || h.BrowserSession.Validate() != nil ||
		len(h.BrowserSession.ipHashKey) != sha256.Size {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil)
	}
	peerIP, err := canonicalPeerIP(c.Request.RemoteAddr)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid client address", err)
	}
	hasher := hmac.New(sha256.New, h.BrowserSession.ipHashKey)
	_, _ = hasher.Write([]byte(peerIP))
	return hasher.Sum(nil), nil
}

func (h *IdentityHandler) oauthCallbackSession(c *gin.Context) {
	if h == nil || c == nil || h.AuthenticationHandler == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	setOAuthCallbackHeaders(c)
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	if !validOAuthTransportProvider(provider) {
		h.redirectOAuthCompletion(c, "error", "", "invalid_callback")
		return
	}
	providerError, providerErrorOK := singularOAuthQuery(c, "error", true)
	if !providerErrorOK {
		h.redirectOAuthCompletion(c, "error", "", "invalid_callback")
		return
	}
	if providerError != "" {
		h.redirectOAuthCompletion(c, "error", "", "provider_denied")
		return
	}
	code, codeOK := singularOAuthQuery(c, "code", false)
	state, stateOK := singularOAuthQuery(c, "state", false)
	if !codeOK || !stateOK {
		h.redirectOAuthCompletion(c, "error", "", "invalid_callback")
		return
	}
	result, err := h.AuthenticationHandler.OAuthCallback(c.Request.Context(), &dto.OAuthCallbackRequest{
		Provider: provider,
		Code:     code,
		State:    state,
	})
	if err != nil || result == nil || result.CompletionTicket == "" ||
		result.TicketID == uuid.Nil || result.ExpiresAt.IsZero() {
		h.redirectOAuthCompletion(c, "error", "", "exchange_failed")
		return
	}
	policy, ok := h.oauthTicketPolicy(result.Purpose)
	if !ok {
		h.redirectOAuthCompletion(c, "error", "", "invalid_callback")
		return
	}
	cookie, err := h.BrowserSession.makeCookie(
		policy,
		result.CompletionTicket,
		result.ExpiresAt,
		true,
	)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err))
		return
	}
	nethttp.SetCookie(c.Writer, cookie)
	h.redirectOAuthCompletion(c, "ready", result.Purpose, "")
}

func (h *IdentityHandler) oauthFinalizeLoginSession(c *gin.Context) {
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	ticket, err := singularCookie(c.Request, h.BrowserSession.OAuthLoginTicket.Name)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeBadRequest, "invalid or expired oauth credential", nil))
		return
	}
	request, err := commonrequest.ParseRequest[oauthFinalizeLoginRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	request.CompletionTicket = ticket.Value
	request.PreSessionID = preSessionID
	if err := h.applySessionCommandMetadata(c, &request.IdempotencyKey, &request.UserAgent, &request.IPHash); err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.AuthenticationHandler.OAuthFinalizeLogin(c.Request.Context(), toOAuthFinalizeLoginRequest(request))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil || result.SessionID == uuid.Nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session issuance", nil))
		return
	}
	if err := h.writeSessionCookies(c, result.SessionID, result.AccessToken, result.AccessTokenExpiresAt, result.RefreshToken, result.RefreshTokenExpiresAt); err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, &successResponse{Success: true})
}

func (h *IdentityHandler) oauthInspectLoginSession(c *gin.Context) {
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	ticket, err := singularCookie(c.Request, h.BrowserSession.OAuthLoginTicket.Name)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeBadRequest, "invalid or expired oauth credential", nil))
		return
	}
	result, err := h.AuthenticationHandler.OAuthInspectLogin(
		c.Request.Context(),
		&dto.OAuthLoginInspectionRequest{
			CompletionTicket: ticket.Value,
			PreSessionID:     preSessionID,
		},
	)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid oauth inspection", nil))
		return
	}
	noStore(c)
	response.Respond(c, toOAuthLoginInspectionResponse(result))
}

func (h *IdentityHandler) oauthFinalizeLinkSession(c *gin.Context) {
	if h == nil || c == nil || h.AuthenticationHandler == nil || h.BrowserSession.Validate() != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	ticket, err := singularCookie(c.Request, h.BrowserSession.OAuthLinkTicket.Name)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeBadRequest, "invalid or expired oauth credential", nil))
		return
	}
	result, err := h.AuthenticationHandler.OAuthFinalizeLink(c.Request.Context(), &dto.OAuthFinalizeLinkRequest{
		CompletionTicket: ticket.Value,
	})
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if result == nil || !result.Success {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid oauth link result", nil))
		return
	}
	nethttp.SetCookie(c.Writer, expiredCookie(h.BrowserSession.OAuthLinkTicket, true))
	noStore(c)
	response.Respond(c, result)
}

func setOAuthCallbackHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Content-Type-Options", "nosniff")
}

func singularOAuthQuery(c *gin.Context, key string, allowEmpty bool) (string, bool) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "", false
	}
	values, exists := c.Request.URL.Query()[key]
	if !exists {
		return "", allowEmpty
	}
	if len(values) != 1 || len(values[0]) > 4096 || strings.TrimSpace(values[0]) != values[0] {
		return "", false
	}
	if values[0] == "" && !allowEmpty {
		return "", false
	}
	return values[0], true
}

func validOAuthTransportProvider(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func (h *IdentityHandler) oauthTicketPolicy(purpose entity.OAuthPurpose) (BrowserCookiePolicy, bool) {
	switch purpose {
	case entity.OAuthPurposeLogin:
		return h.BrowserSession.OAuthLoginTicket, true
	case entity.OAuthPurposeLink:
		return h.BrowserSession.OAuthLinkTicket, true
	default:
		return BrowserCookiePolicy{}, false
	}
}

func (h *IdentityHandler) redirectOAuthCompletion(c *gin.Context, status string, purpose entity.OAuthPurpose, reason string) {
	target, err := url.Parse(h.BrowserSession.OAuthCompletionURL)
	if err != nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", err))
		return
	}
	query := target.Query()
	query.Set("status", status)
	if purpose.Valid() {
		query.Set("purpose", string(purpose))
	}
	if reason != "" {
		query.Set("reason", reason)
	}
	target.RawQuery = query.Encode()
	c.Redirect(nethttp.StatusSeeOther, target.String())
}

func parseIdempotencyKey(value string) (uuid.UUID, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return uuid.Nil, apperr.New(apperr.CodeBadRequest, "valid Idempotency-Key header is required", nil)
	}
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, apperr.New(apperr.CodeBadRequest, "valid Idempotency-Key header is required", err)
	}
	return parsed, nil
}

func parseIdempotencyHeader(c *gin.Context) (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, apperr.New(apperr.CodeBadRequest, "valid Idempotency-Key header is required", nil)
	}
	values := c.Request.Header.Values(commonrequest.IdempotencyKeyHeader)
	if len(values) != 1 {
		return uuid.Nil, apperr.New(apperr.CodeBadRequest, "valid Idempotency-Key header is required", nil)
	}
	return parseIdempotencyKey(values[0])
}

func validTransportText(value string, maximum int) bool {
	if !utf8.ValidString(value) || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) || unicode.Is(unicode.Bidi_Control, character) {
			return false
		}
	}
	return true
}

func (h *IdentityHandler) requireTrustedBrowserMutation(c *gin.Context) error {
	if h == nil || c == nil || h.BrowserSession.Validate() != nil {
		return apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil)
	}
	origins := c.Request.Header.Values("Origin")
	if len(origins) != 1 {
		return apperr.New(apperr.CodeForbidden, "untrusted browser origin", nil)
	}
	origin, err := normalizeBrowserOrigin(origins[0])
	if err != nil || !h.BrowserSession.allowsOrigin(origin) {
		return apperr.New(apperr.CodeForbidden, "untrusted browser origin", nil)
	}
	switch fetchSite := strings.ToLower(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site"))); fetchSite {
	case "", "same-origin", "same-site", "none":
		return nil
	default:
		return apperr.New(apperr.CodeForbidden, "untrusted browser request context", nil)
	}
}

func normalizeBrowserOrigin(raw string) (string, error) {
	if raw == "" || len(raw) > 2048 || strings.TrimSpace(raw) != raw {
		return "", errors.New("invalid browser origin")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.User != nil || parsed.Host == "" ||
		parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid browser origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("invalid browser origin")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return "", errors.New("invalid browser origin")
	}
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	return scheme + "://" + host, nil
}

func canonicalPeerIP(remoteAddress string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	parsed := net.ParseIP(strings.Trim(host, "[]"))
	if parsed == nil {
		return "", errors.New("invalid peer address")
	}
	return parsed.String(), nil
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func identityHTTPFail(c *gin.Context, err error) {
	noStore(c)
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
}
