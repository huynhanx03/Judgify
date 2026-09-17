package http

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
)

// SessionManagementHandler is intentionally narrower than the authentication
// handler: account security UI operations can evolve without expanding the
// token issuance/authentication interface.
type SessionManagementHandler interface {
	List(context.Context, *sessionListQuery) (*sessionListResponse, error)
	Revoke(context.Context, uuid.UUID, uuid.UUID) error
	LogoutAll(context.Context, uuid.UUID) error
}

type sessionManagementHandler struct {
	lifecycle ports.SessionManagementLifecycle
}

func NewSessionManagementHandler(lifecycle ports.SessionManagementLifecycle) SessionManagementHandler {
	return &sessionManagementHandler{lifecycle: lifecycle}
}

func (handler *sessionManagementHandler) List(
	ctx context.Context,
	request *sessionListQuery,
) (*sessionListResponse, error) {
	userID, sessionID, err := canonicalSessionPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.lifecycle == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "session management unavailable", nil)
	}
	result, err := handler.lifecycle.List(ctx, userID, sessionID, toSessionListRequest(request))
	if err != nil {
		return nil, err
	}
	return toSessionListResponse(result), nil
}

func (handler *sessionManagementHandler) Revoke(
	ctx context.Context,
	targetSessionID, idempotencyKey uuid.UUID,
) error {
	userID, actorSessionID, err := canonicalSessionPrincipal(ctx)
	if err != nil {
		return err
	}
	if handler == nil || handler.lifecycle == nil {
		return apperr.New(apperr.CodeInternalServer, "session management unavailable", nil)
	}
	return handler.lifecycle.Revoke(ctx, userID, actorSessionID, targetSessionID, idempotencyKey)
}

func (handler *sessionManagementHandler) LogoutAll(ctx context.Context, idempotencyKey uuid.UUID) error {
	userID, actorSessionID, err := canonicalSessionPrincipal(ctx)
	if err != nil {
		return err
	}
	if handler == nil || handler.lifecycle == nil {
		return apperr.New(apperr.CodeInternalServer, "session management unavailable", nil)
	}
	return handler.lifecycle.LogoutAll(ctx, userID, actorSessionID, idempotencyKey)
}

func (h *IdentityHandler) registerSessionManagementRoutes(r *gin.RouterGroup) {
	if h == nil || r == nil {
		return
	}
	auth := r.Group("/auth")
	auth.Use(noStore)
	{
		auth.GET("/sessions", h.listSessions)
		auth.DELETE("/sessions/:id", h.revokeSession)
		auth.POST("/logout-all", h.logoutAllSessions)
	}
}

func (h *IdentityHandler) listSessions(c *gin.Context) {
	if h == nil || h.SessionManagementHandler == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "session management unavailable", nil))
		return
	}
	request, err := parseSessionListQuery(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.SessionManagementHandler.List(c.Request.Context(), request)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	response.Respond(c, result)
}

func (h *IdentityHandler) revokeSession(c *gin.Context) {
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	targetID, err := parseCanonicalSessionID(c.Param("id"))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if h == nil || h.SessionManagementHandler == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "session management unavailable", nil))
		return
	}
	if err := h.SessionManagementHandler.Revoke(c.Request.Context(), targetID, idempotencyKey); err != nil {
		identityHTTPFail(c, err)
		return
	}
	if currentID, ok := authctx.SessionID(c.Request.Context()); ok && currentID == targetID {
		h.clearSessionCookies(c)
	}
	response.Respond(c, &sessionMutationResponse{Success: true})
}

func (h *IdentityHandler) logoutAllSessions(c *gin.Context) {
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if h == nil || h.SessionManagementHandler == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "session management unavailable", nil))
		return
	}
	if err := h.SessionManagementHandler.LogoutAll(c.Request.Context(), idempotencyKey); err != nil {
		identityHTTPFail(c, err)
		return
	}
	h.clearSessionCookies(c)
	response.Respond(c, &sessionMutationResponse{Success: true})
}

func parseSessionListQuery(c *gin.Context) (*sessionListQuery, error) {
	if c == nil || c.Request == nil {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session query", nil)
	}
	if len(c.Request.URL.RawQuery) > 1024 {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session query", nil)
	}
	query := c.Request.URL.Query()
	for key := range query {
		if key != "cursor" && key != "limit" {
			return nil, apperr.New(apperr.CodeBadRequest, "invalid session query", nil)
		}
	}
	cursorValues, cursorPresent := query["cursor"]
	if cursorPresent && (len(cursorValues) != 1 || cursorValues[0] == "") {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session cursor", nil)
	}
	limitValues, limitPresent := query["limit"]
	if limitPresent && (len(limitValues) != 1 || limitValues[0] == "") {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session limit", nil)
	}
	request := &sessionListQuery{}
	if cursorPresent {
		request.Cursor = cursorValues[0]
	}
	if !limitPresent {
		return request, nil
	}
	rawLimit := limitValues[0]
	if strings.TrimSpace(rawLimit) != rawLimit {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session limit", nil)
	}
	limit, err := strconv.Atoi(rawLimit)
	if err != nil || limit < 1 || limit > 50 {
		return nil, apperr.New(apperr.CodeBadRequest, "invalid session limit", err)
	}
	request.Limit = limit
	return request, nil
}

func parseCanonicalSessionID(raw string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed == uuid.Nil || parsed.String() != raw {
		return uuid.Nil, apperr.New(apperr.CodeBadRequest, "invalid session id", err)
	}
	return parsed, nil
}

func canonicalSessionPrincipal(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return uuid.Nil, uuid.Nil, apperr.New(apperr.CodeUnauthorized, "authenticated session required", nil)
	}
	return userID, sessionID, nil
}

var _ SessionManagementHandler = (*sessionManagementHandler)(nil)
