// Package operationhttp exposes the transport-neutral operation control plane
// through the product HTTP API. Keeping Gin here prevents workers that execute
// operations from linking the product router.
package operationhttp

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/platform/operation"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

const maxRawQueryBytes = 2048

var ErrInvalidHandler = errors.New("operation http: invalid handler")

type Reader interface {
	Get(context.Context, uuid.UUID) (operation.Operation, error)
	List(context.Context, operation.ListFilter) (operation.Page, error)
}

type Controller interface {
	Control(context.Context, operation.ControlCommand) (operation.ControlResult, error)
}

type Handler struct {
	reader      Reader
	batchReader operation.OperationBatchReader
	controller  Controller
}

func NewHandler(reader Reader, controller Controller) (*Handler, error) {
	batchReader, ok := reader.(operation.OperationBatchReader)
	if reader == nil || controller == nil || !ok || batchReader == nil {
		return nil, ErrInvalidHandler
	}
	return &Handler{
		reader:      reader,
		batchReader: batchReader,
		controller:  controller,
	}, nil
}

func (handler *Handler) RegisterProtected(
	router *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	if handler == nil || router == nil || permissionChecker == nil {
		return
	}
	group := router.Group("/admin/operations")
	group.Use(noStore)
	group.GET(
		"",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyOperation,
			permissions.PermissionScopeRead,
		),
		handler.list,
	)
	group.GET(
		"/:id",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyOperation,
			permissions.PermissionScopeRead,
		),
		handler.get,
	)
	group.GET(
		"/:id/batches",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyOperation,
			permissions.PermissionScopeRead,
		),
		handler.listBatches,
	)
	group.POST(
		"/:id/control",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyOperation,
			permissions.PermissionScopeExecute,
		),
		handler.control,
	)
}

type getRequest struct {
	ID string `uri:"id" validate:"required,max=36"`
}

type controlRequest struct {
	ID              string                  `uri:"id" validate:"required,max=36"`
	Action          operation.ControlAction `json:"action" validate:"required,oneof=pause resume cancel"`
	ExpectedVersion int64                   `json:"expected_version" validate:"required,min=1"`
	Reason          string                  `json:"reason" validate:"required,max=1024"`
}

func (handler *Handler) listBatches(c *gin.Context) {
	if handler == nil || handler.reader == nil || handler.batchReader == nil ||
		c == nil || c.Request == nil {
		fail(c, ErrInvalidHandler)
		return
	}
	request, err := commonrequest.ParseRequest[getRequest](c)
	if err != nil {
		fail(c, err)
		return
	}
	operationID, err := parseCanonicalUUID(request.ID)
	if err != nil {
		fail(c, err)
		return
	}
	filter, err := parseBatchListQuery(c.Request.URL.RawQuery)
	if err != nil {
		fail(c, err)
		return
	}
	if _, err := handler.reader.Get(c.Request.Context(), operationID); err != nil {
		fail(c, err)
		return
	}
	page, err := handler.batchReader.ListBatches(c.Request.Context(), operationID, filter)
	if err != nil {
		fail(c, err)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, page)
}

func (handler *Handler) get(c *gin.Context) {
	if handler == nil || handler.reader == nil || c == nil || c.Request == nil {
		fail(c, ErrInvalidHandler)
		return
	}
	request, err := commonrequest.ParseRequest[getRequest](c)
	if err != nil {
		fail(c, err)
		return
	}
	id, err := parseCanonicalUUID(request.ID)
	if err != nil {
		fail(c, err)
		return
	}
	value, err := handler.reader.Get(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	projected, err := operation.Project(value)
	if err != nil {
		fail(c, err)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, projected)
}

func (handler *Handler) list(c *gin.Context) {
	if handler == nil || handler.reader == nil || c == nil || c.Request == nil {
		fail(c, ErrInvalidHandler)
		return
	}
	filter, err := parseListQuery(c.Request.URL.RawQuery)
	if err != nil {
		fail(c, err)
		return
	}
	page, err := handler.reader.List(c.Request.Context(), filter)
	if err != nil {
		fail(c, err)
		return
	}
	projected, err := operation.ProjectPage(page)
	if err != nil {
		fail(c, err)
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, projected)
}

func (handler *Handler) control(c *gin.Context) {
	if handler == nil || handler.controller == nil || c == nil || c.Request == nil {
		fail(c, ErrInvalidHandler)
		return
	}
	request, err := commonrequest.ParseRequest[controlRequest](c)
	if err != nil {
		fail(c, err)
		return
	}
	operationID, err := parseCanonicalUUID(request.ID)
	if err != nil {
		fail(c, err)
		return
	}
	idempotencyKey, err := strictIdempotencyHeader(c)
	if err != nil {
		fail(c, err)
		return
	}
	actorUserID, userOK := authctx.UserID(c.Request.Context())
	actorSessionID, sessionOK := authctx.SessionID(c.Request.Context())
	if !userOK || !sessionOK {
		fail(c, apperr.New(apperr.CodeUnauthorized, "authentication required", nil))
		return
	}
	result, err := handler.controller.Control(
		c.Request.Context(),
		operation.ControlCommand{
			OperationID:     operationID,
			Action:          request.Action,
			ExpectedVersion: request.ExpectedVersion,
			ActorUserID:     actorUserID,
			ActorSessionID:  actorSessionID,
			IdempotencyKey:  idempotencyKey,
			Reason:          request.Reason,
		},
	)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Idempotency-Replayed", strconv.FormatBool(result.Replay))
	response.SuccessResponse(c, apperr.CodeSuccess, result)
}

func parseListQuery(raw string) (operation.ListFilter, error) {
	if len(raw) > maxRawQueryBytes {
		return operation.ListFilter{}, operation.ErrInvalidOperation
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return operation.ListFilter{}, operation.ErrInvalidOperation
	}
	for key, candidates := range values {
		if len(candidates) != 1 || candidates[0] == "" {
			return operation.ListFilter{}, operation.ErrInvalidOperation
		}
		switch key {
		case "status", "kind", "actor_id", "cursor", "limit":
		default:
			return operation.ListFilter{}, operation.ErrInvalidOperation
		}
	}
	filter := operation.ListFilter{Limit: 25}
	if value := values.Get("status"); value != "" {
		filter.Status = operation.Status(value)
	}
	if value := values.Get("kind"); value != "" {
		filter.Kind = value
	}
	if value := values.Get("actor_id"); value != "" {
		filter.ActorID, err = parseCanonicalUUID(value)
		if err != nil {
			return operation.ListFilter{}, err
		}
	}
	if value := values.Get("cursor"); value != "" {
		filter.BeforeAt, filter.BeforeID, err = operation.DecodeCursor(value)
		if err != nil {
			return operation.ListFilter{}, err
		}
	}
	if value := values.Get("limit"); value != "" {
		filter.Limit, err = strconv.Atoi(value)
		if err != nil {
			return operation.ListFilter{}, operation.ErrInvalidOperation
		}
	}
	if err := operation.ValidateListFilter(filter); err != nil {
		return operation.ListFilter{}, err
	}
	return filter, nil
}

func parseBatchListQuery(raw string) (operation.BatchListFilter, error) {
	if len(raw) > maxRawQueryBytes {
		return operation.BatchListFilter{}, operation.ErrInvalidOperation
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return operation.BatchListFilter{}, operation.ErrInvalidOperation
	}
	for key, candidates := range values {
		if len(candidates) != 1 || candidates[0] == "" {
			return operation.BatchListFilter{}, operation.ErrInvalidOperation
		}
		switch key {
		case "cursor", "limit":
		default:
			return operation.BatchListFilter{}, operation.ErrInvalidOperation
		}
	}
	filter := operation.BatchListFilter{Limit: 25}
	if cursor := values.Get("cursor"); cursor != "" {
		filter.BeforeSequence, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil || filter.BeforeSequence < 2 {
			return operation.BatchListFilter{}, operation.ErrInvalidOperation
		}
	}
	if limit := values.Get("limit"); limit != "" {
		filter.Limit, err = strconv.Atoi(limit)
		if err != nil {
			return operation.BatchListFilter{}, operation.ErrInvalidOperation
		}
	}
	if err := operation.ValidateBatchListFilter(filter); err != nil {
		return operation.BatchListFilter{}, err
	}
	return filter, nil
}

func strictIdempotencyHeader(c *gin.Context) (uuid.UUID, error) {
	if c == nil || c.Request == nil {
		return uuid.Nil, operation.ErrInvalidOperation
	}
	values := c.Request.Header.Values(commonrequest.IdempotencyKeyHeader)
	if len(values) != 1 {
		return uuid.Nil, operation.ErrInvalidOperation
	}
	return parseCanonicalUUID(values[0])
}

func parseCanonicalUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, operation.ErrInvalidOperation
	}
	return parsed, nil
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Next()
}

func fail(c *gin.Context, err error) {
	if c == nil {
		return
	}
	switch {
	case errors.Is(err, operation.ErrNotFound):
		err = apperr.New(apperr.CodeNotFound, "operation not found", err)
	case errors.Is(err, operation.ErrConflict), errors.Is(err, operation.ErrLeaseLost):
		err = apperr.New(apperr.CodeConflict, "operation state changed", err)
	case errors.Is(err, operation.ErrInvalidOperation),
		errors.Is(err, operation.ErrInvalidCheckpoint),
		errors.Is(err, operation.ErrInvalidRequest),
		errors.Is(err, operation.ErrInvalidBatchResult):
		err = apperr.New(apperr.CodeBadRequest, "invalid operation request", err)
	default:
		err = apperr.New(apperr.CodeInternalServer, "operation unavailable", err)
	}
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}
