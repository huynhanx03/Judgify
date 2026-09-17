package http

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonhandler "github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	contract "github.com/huynhanx03/judgify/internal/notification/contract"
	notificationservice "github.com/huynhanx03/judgify/internal/notification/usecase"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type CampaignHandler struct {
	service   ports.CampaignService
	scheduler ports.CampaignScheduler
	canceller ports.CampaignCanceller
}

func NewCampaignHandler(
	service ports.CampaignService,
	scheduler ports.CampaignScheduler,
	canceller ports.CampaignCanceller,
) (*CampaignHandler, error) {
	if service == nil || scheduler == nil || canceller == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &CampaignHandler{
		service:   service,
		scheduler: scheduler,
		canceller: canceller,
	}, nil
}

func (handler *CampaignHandler) RegisterProtected(
	router *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	if handler == nil || router == nil || permissionChecker == nil {
		return
	}
	group := router.Group("/admin/notification-campaigns")
	group.Use(noStore)
	compose := permissionChecker.RequirePermission(
		permissions.ResourceKeyNotification,
		permissions.PermissionScopeCompose,
	)
	readDelivery := permissionChecker.RequirePermission(
		permissions.ResourceKeyNotification,
		permissions.PermissionScopeReadDelivery,
	)
	group.GET("", compose, commonhandler.Wrap(handler.List))
	group.POST("", compose, commonhandler.Wrap(handler.Create))
	group.GET("/:id", compose, commonhandler.Wrap(handler.Get))
	group.PUT("/:id", compose, commonhandler.Wrap(handler.Revise))
	group.POST("/:id/dry-run", compose, commonhandler.Wrap(handler.DryRun))
	group.POST("/:id/schedule", compose, handler.Schedule)
	group.POST("/:id/cancel", compose, handler.Cancel)
	group.GET(
		"/:id/recipients",
		readDelivery,
		commonhandler.Wrap(handler.ListRecipients),
	)
}

func (handler *CampaignHandler) Create(
	ctx context.Context,
	request *contract.CreateCampaignRequest,
) (*contract.CampaignResponse, error) {
	actorID, err := campaignActor(ctx)
	if err != nil {
		return nil, err
	}
	return handler.service.Create(ctx, actorID, request)
}

func (handler *CampaignHandler) Revise(
	ctx context.Context,
	request *contract.ReviseCampaignRequest,
) (*contract.CampaignResponse, error) {
	actorID, err := campaignActor(ctx)
	if err != nil {
		return nil, err
	}
	return handler.service.Revise(ctx, actorID, request)
}

func (handler *CampaignHandler) Get(
	ctx context.Context,
	request *contract.CampaignIDRequest,
) (*contract.CampaignResponse, error) {
	return handler.service.Get(ctx, request.ID)
}

func (handler *CampaignHandler) List(
	ctx context.Context,
	request *contract.CampaignListRequest,
) (*contract.CampaignListResponse, error) {
	return handler.service.List(ctx, request)
}

func (handler *CampaignHandler) DryRun(
	ctx context.Context,
	request *contract.DryRunCampaignRequest,
) (*contract.CampaignDryRunResponse, error) {
	actorID, err := campaignActor(ctx)
	if err != nil {
		return nil, err
	}
	sessionID, ok := authctx.SessionID(ctx)
	if !ok {
		return nil, campaignAuthenticationError()
	}
	return handler.service.DryRun(ctx, actorID, sessionID, request)
}

func (handler *CampaignHandler) ListRecipients(
	ctx context.Context,
	request *contract.CampaignRecipientListRequest,
) (*contract.CampaignRecipientListResponse, error) {
	return handler.service.ListRecipients(ctx, request)
}

func (handler *CampaignHandler) Schedule(c *gin.Context) {
	if handler == nil || handler.scheduler == nil || c == nil ||
		c.Request == nil {
		failCampaignHTTP(c, ports.ErrInvalidRequest)
		return
	}
	request, err := commonrequest.ParseRequest[contract.ScheduleCampaignRequest](c)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	idempotencyKey, err := campaignIdempotencyKey(c)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	actorID, actorOK := authctx.UserID(c.Request.Context())
	sessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !actorOK || !sessionOK {
		failCampaignHTTP(c, campaignAuthenticationError())
		return
	}
	result, err := handler.scheduler.Schedule(
		c.Request.Context(),
		actorID,
		sessionID,
		idempotencyKey,
		request,
	)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	c.Header("Idempotency-Replayed", strconv.FormatBool(result.Replayed))
	response.SuccessResponse(c, apperr.CodeSuccess, result)
}

func (handler *CampaignHandler) Cancel(c *gin.Context) {
	if handler == nil || handler.canceller == nil || c == nil ||
		c.Request == nil {
		failCampaignHTTP(c, ports.ErrInvalidRequest)
		return
	}
	request, err := commonrequest.ParseRequest[contract.CancelCampaignRequest](c)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	idempotencyKey, err := campaignIdempotencyKey(c)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	actorID, actorOK := authctx.UserID(c.Request.Context())
	sessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !actorOK || !sessionOK {
		failCampaignHTTP(c, campaignAuthenticationError())
		return
	}
	result, err := handler.canceller.Cancel(
		c.Request.Context(),
		actorID,
		sessionID,
		idempotencyKey,
		request,
	)
	if err != nil {
		failCampaignHTTP(c, err)
		return
	}
	c.Header("Idempotency-Replayed", strconv.FormatBool(result.Replayed))
	response.SuccessResponse(c, apperr.CodeSuccess, result)
}

func campaignActor(ctx context.Context) (uuid.UUID, error) {
	actorID, ok := authctx.UserID(ctx)
	if !ok {
		return uuid.Nil, campaignAuthenticationError()
	}
	return actorID, nil
}

func campaignIdempotencyKey(c *gin.Context) (uuid.UUID, error) {
	values := c.Request.Header.Values(commonrequest.IdempotencyKeyHeader)
	if len(values) != 1 {
		return uuid.Nil, apperr.New(
			apperr.CodeParamInvalid,
			"one canonical idempotency key is required",
			ports.ErrInvalidRequest,
		)
	}
	value, err := uuid.Parse(values[0])
	if err != nil || value == uuid.Nil || value.String() != values[0] {
		return uuid.Nil, apperr.New(
			apperr.CodeParamInvalid,
			"invalid idempotency key",
			ports.ErrInvalidRequest,
		)
	}
	return value, nil
}

func campaignAuthenticationError() error {
	return apperr.New(
		apperr.CodeUnauthorized,
		"authentication required",
		nil,
	)
}

func failCampaignHTTP(c *gin.Context, err error) {
	if c == nil {
		return
	}
	err = notificationservice.MapCampaignError(err)
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}
