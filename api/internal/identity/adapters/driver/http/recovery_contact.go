package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

type RecoveryContactHandler interface {
	Verify(
		context.Context,
		*verifyRecoveryContactRequest,
	) (*verifyRecoveryContactResponse, error)
	Get(context.Context) (*recoveryContactResponse, error)
	Reauthenticate(
		context.Context,
		*reauthenticateRecoveryContactRequest,
	) (*reauthenticateRecoveryContactResponse, error)
	Replace(context.Context, *replaceRecoveryContactRequest) (*recoveryContactResponse, error)
	Resend(context.Context, *resendRecoveryContactRequest) (*recoveryContactResponse, error)
	CancelPending(context.Context, *cancelPendingRecoveryContactRequest) (*recoveryContactResponse, error)
	Remove(context.Context, *removeRecoveryContactRequest) (*recoveryContactResponse, error)
}

type recoveryContactHandler struct {
	application ports.RecoveryContactApplication
}

func NewRecoveryContactHandler(application ports.RecoveryContactApplication) RecoveryContactHandler {
	return &recoveryContactHandler{application: application}
}

func (handler *recoveryContactHandler) Get(
	ctx context.Context,
) (*recoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	projection, err := handler.application.Get(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return toRecoveryContactResponse(projection)
}

func (handler *recoveryContactHandler) Reauthenticate(
	ctx context.Context,
	request *reauthenticateRecoveryContactRequest,
) (*reauthenticateRecoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	until, err := handler.application.Reauthenticate(ctx, userID, sessionID, request.CurrentPassword)
	if err != nil {
		return nil, err
	}
	return &reauthenticateRecoveryContactResponse{
		Status: reauthenticationStatusVerified, ReauthenticatedUntil: until,
	}, nil
}

func (handler *recoveryContactHandler) Replace(
	ctx context.Context,
	request *replaceRecoveryContactRequest,
) (*recoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	projection, err := handler.application.Replace(
		ctx, userID, sessionID, request.IdempotencyKey,
		request.RecoveryEmail, request.ExpectedCurrentVersion,
	)
	if err != nil {
		return nil, err
	}
	return toRecoveryContactResponse(projection)
}

func (handler *recoveryContactHandler) Resend(
	ctx context.Context,
	request *resendRecoveryContactRequest,
) (*recoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	projection, err := handler.application.Resend(
		ctx, userID, sessionID, request.IdempotencyKey, request.ExpectedPendingVersion,
	)
	if err != nil {
		return nil, err
	}
	return toRecoveryContactResponse(projection)
}

func (handler *recoveryContactHandler) CancelPending(
	ctx context.Context,
	request *cancelPendingRecoveryContactRequest,
) (*recoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	projection, err := handler.application.CancelPending(
		ctx, userID, sessionID, request.IdempotencyKey, request.ExpectedPendingVersion,
	)
	if err != nil {
		return nil, err
	}
	return toRecoveryContactResponse(projection)
}

func (handler *recoveryContactHandler) Remove(
	ctx context.Context,
	request *removeRecoveryContactRequest,
) (*recoveryContactResponse, error) {
	userID, sessionID, err := recoveryContactPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	projection, err := handler.application.Remove(
		ctx, userID, sessionID, request.IdempotencyKey, request.ExpectedCurrentVersion,
	)
	if err != nil {
		return nil, err
	}
	return toRecoveryContactResponse(projection)
}

func recoveryContactPrincipal(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	userID, userOK := authctx.UserID(ctx)
	sessionID, sessionOK := authctx.SessionID(ctx)
	if !userOK || !sessionOK {
		return uuid.Nil, uuid.Nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	return userID, sessionID, nil
}

func toRecoveryContactResponse(
	projection *entity.RecoveryContactProjection,
) (*recoveryContactResponse, error) {
	if projection == nil {
		return nil, apperr.New(apperr.CodeInternalServer, "authentication service unavailable", nil)
	}
	result := &recoveryContactResponse{
		ReauthenticatedUntil: projection.ReauthenticatedUntil,
		CanRemove:            projection.CanRemove,
	}
	if projection.Current != nil {
		result.Current = &recoveryContactCurrentResponse{
			Status: string(projection.Current.Status), MaskedAddress: projection.Current.MaskedAddress,
			Version: projection.Current.Version,
		}
	}
	if projection.Pending != nil {
		result.Pending = &recoveryContactPendingResponse{
			Purpose: string(projection.Pending.Purpose), Status: string(projection.Pending.Status),
			MaskedAddress: projection.Pending.MaskedAddress, Version: projection.Pending.Version,
			VerificationExpiresAt: projection.Pending.VerificationExpiresAt,
			ResendAvailableAt:     projection.Pending.ResendAvailableAt,
			DeliveryState:         string(projection.Pending.DeliveryState),
		}
	}
	return result, nil
}

func (h *IdentityHandler) registerRecoveryContactRoutes(r *gin.RouterGroup) {
	group := r.Group("/auth")
	group.Use(noStore)
	group.GET("/recovery-contact", handler.WrapNoReq(h.RecoveryContactHandler.Get))
	group.POST("/reauthenticate", h.reauthenticateRecoveryContact)
	group.PUT("/recovery-contact", h.replaceRecoveryContact)
	group.POST("/recovery-contact/resend", h.resendRecoveryContact)
	group.DELETE("/recovery-contact/pending", h.cancelPendingRecoveryContact)
	group.DELETE("/recovery-contact", h.removeRecoveryContact)
}

func (h *IdentityHandler) verifyRecoveryContactMutation(c *gin.Context) {
	if _, err := h.requirePreSessionMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[verifyRecoveryContactRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.RecoveryContactHandler.Verify(c.Request.Context(), request)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, result)
}

func (h *IdentityHandler) replaceRecoveryContact(c *gin.Context) {
	executeRecoveryContactMutation(
		h, c,
		func(request *replaceRecoveryContactRequest, key uuid.UUID) { request.IdempotencyKey = key },
		h.RecoveryContactHandler.Replace,
	)
}

func (h *IdentityHandler) resendRecoveryContact(c *gin.Context) {
	executeRecoveryContactMutation(
		h, c,
		func(request *resendRecoveryContactRequest, key uuid.UUID) { request.IdempotencyKey = key },
		h.RecoveryContactHandler.Resend,
	)
}

func (h *IdentityHandler) cancelPendingRecoveryContact(c *gin.Context) {
	executeRecoveryContactMutation(
		h, c,
		func(request *cancelPendingRecoveryContactRequest, key uuid.UUID) { request.IdempotencyKey = key },
		h.RecoveryContactHandler.CancelPending,
	)
}

func (h *IdentityHandler) removeRecoveryContact(c *gin.Context) {
	executeRecoveryContactMutation(
		h, c,
		func(request *removeRecoveryContactRequest, key uuid.UUID) { request.IdempotencyKey = key },
		h.RecoveryContactHandler.Remove,
	)
}

func executeRecoveryContactMutation[Request any](
	h *IdentityHandler,
	c *gin.Context,
	setIdempotencyKey func(*Request, uuid.UUID),
	invoke func(context.Context, *Request) (*recoveryContactResponse, error),
) {
	if h == nil || c == nil || setIdempotencyKey == nil || invoke == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "authentication transport unavailable", nil))
		return
	}
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[Request](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	setIdempotencyKey(request, idempotencyKey)
	result, err := invoke(c.Request.Context(), request)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, result)
}

func (h *IdentityHandler) reauthenticateRecoveryContact(c *gin.Context) {
	if err := h.requireTrustedBrowserMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	// Reauthentication is harmless to replay, but requiring one canonical key
	// keeps every security mutation on the same unambiguous transport contract.
	if _, err := parseIdempotencyHeader(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[reauthenticateRecoveryContactRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.RecoveryContactHandler.Reauthenticate(c.Request.Context(), request)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, result)
}

func (handler *recoveryContactHandler) Verify(
	ctx context.Context,
	request *verifyRecoveryContactRequest,
) (*verifyRecoveryContactResponse, error) {
	if handler == nil || handler.application == nil || request == nil {
		return nil, apperr.New(
			apperr.CodeInternalServer,
			"authentication service unavailable",
			nil,
		)
	}
	if _, err := handler.application.Verify(ctx, request.Token); err != nil {
		return nil, err
	}
	return &verifyRecoveryContactResponse{
		Status: recoveryContactVerificationStatusVerified,
	}, nil
}
