package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonhandler "github.com/huynhanx03/go-common/pkg/common/http/handler"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
)

type Handler struct {
	service ports.Service
}

func NewHandler(service ports.Service) (*Handler, error) {
	if service == nil {
		return nil, ports.ErrInvalidRequest
	}
	return &Handler{service: service}, nil
}

func (handler *Handler) RegisterProtected(router *gin.RouterGroup) {
	notifications := router.Group("/notifications")
	notifications.Use(noStore)
	{
		notifications.GET("", commonhandler.Wrap(handler.List))
		notifications.GET("/unread", commonhandler.WrapNoReq(handler.Unread))
		notifications.GET("/:id/content", commonhandler.Wrap(handler.CampaignContent))
		notifications.POST("/read-all", commonhandler.WrapNoReq(handler.MarkAllRead))
		notifications.POST("/:id/read", commonhandler.Wrap(handler.MarkRead))
		notifications.DELETE("/:id", commonhandler.Wrap(handler.Archive))
	}

	preferences := router.Group("/notification-preferences")
	preferences.Use(noStore)
	{
		preferences.GET("", commonhandler.WrapNoReq(handler.Preferences))
		preferences.PUT("/:category", commonhandler.Wrap(handler.UpdatePreference))
	}
}

func (handler *Handler) List(ctx context.Context, request *notificationListRequest) (*notificationListResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.List(ctx, userID, toNotificationListQuery(request))
	if err != nil {
		return nil, err
	}
	return toNotificationListResponse(result), nil
}

func (handler *Handler) Unread(ctx context.Context) (*unreadCountResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.Unread(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUnreadCountResponse(result), nil
}

func (handler *Handler) CampaignContent(
	ctx context.Context,
	request *notificationIDRequest,
) (*campaignContentResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := toNotificationID(request)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.CampaignContent(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toCampaignContentResponse(result), nil
}

func (handler *Handler) MarkRead(
	ctx context.Context,
	request *notificationIDRequest,
) (*notificationMutationResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := toNotificationID(request)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.MarkRead(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toNotificationMutationResponse(result), nil
}

func (handler *Handler) MarkAllRead(ctx context.Context) (*notificationMutationResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.MarkAllRead(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toNotificationMutationResponse(result), nil
}

func (handler *Handler) Archive(
	ctx context.Context,
	request *notificationIDRequest,
) (*notificationMutationResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := toNotificationID(request)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.Archive(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toNotificationMutationResponse(result), nil
}

func (handler *Handler) Preferences(ctx context.Context) ([]preferenceResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.Preferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toPreferenceResponses(result), nil
}

func (handler *Handler) UpdatePreference(
	ctx context.Context,
	request *updateNotificationPreferenceRequest,
) (*preferenceResponse, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := handler.service.UpdatePreference(ctx, userID, toUpdateNotificationPreferenceRequest(request))
	if err != nil {
		return nil, err
	}
	return toPreferenceResponse(result), nil
}

func authenticatedUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := authctx.UserID(ctx)
	if !ok {
		return uuid.Nil, apperr.New(apperr.CodeUnauthorized, "authentication required", nil)
	}
	return userID, nil
}

func noStore(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Header("Pragma", "no-cache")
	ctx.Next()
}
