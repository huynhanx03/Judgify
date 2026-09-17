package debuglevel

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"
	commonobservability "github.com/huynhanx03/go-common/pkg/common/observability"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

type HTTPHandler struct {
	workflow  *Workflow
	component string
}

type activateRequest struct {
	DurationSeconds int64  `json:"duration_seconds" validate:"required,min=1,max=3600"`
	Reason          string `json:"reason" validate:"required,max=1024"`
}

type deactivateRequest struct {
	Key    string `uri:"key" validate:"required,max=36"`
	Reason string `json:"reason" validate:"required,max=1024"`
}

type debugWindowProjection struct {
	Active    bool       `json:"active"`
	Key       string     `json:"key,omitempty"`
	Component string     `json:"component,omitempty"`
	Level     string     `json:"level,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func NewHTTPHandler(
	workflow *Workflow,
	component string,
) (*HTTPHandler, error) {
	if workflow == nil || !validToken(component, maxComponentBytes) {
		return nil, ErrInvalidCommand
	}
	return &HTTPHandler{workflow: workflow, component: component}, nil
}

func (handler *HTTPHandler) RegisterProtected(
	router *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	if handler == nil || router == nil || permissionChecker == nil {
		return
	}
	group := router.Group("/admin/observability")
	group.Use(noStore)
	group.GET(
		"/debug-window",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyObservability,
			permissions.PermissionScopeRead,
		),
		handler.active,
	)
	group.POST(
		"/debug-window",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyObservability,
			permissions.PermissionScopeUpdate,
		),
		handler.activate,
	)
	group.DELETE(
		"/debug-window/:key",
		permissionChecker.RequirePermission(
			permissions.ResourceKeyObservability,
			permissions.PermissionScopeUpdate,
		),
		handler.deactivate,
	)
}

func (handler *HTTPHandler) active(c *gin.Context) {
	if !validHTTPHandler(handler, c) {
		failHTTP(c, ErrInvalidCommand)
		return
	}
	lease, active, err := handler.workflow.Active(c.Request.Context())
	if err != nil {
		failHTTP(c, err)
		return
	}
	response.SuccessResponse(
		c,
		apperr.CodeSuccess,
		projectDebugWindow(lease, active),
	)
}

func (handler *HTTPHandler) activate(c *gin.Context) {
	if !validHTTPHandler(handler, c) {
		failHTTP(c, ErrInvalidCommand)
		return
	}
	request, err := commonrequest.ParseRequest[activateRequest](c)
	if err != nil {
		failHTTP(c, err)
		return
	}
	idempotencyKey, err := strictIdempotencyKey(c)
	if err != nil {
		failHTTP(c, err)
		return
	}
	lease, active, err := handler.workflow.Activate(c.Request.Context(), Command{
		Component:      handler.component,
		Level:          "debug",
		Duration:       time.Duration(request.DurationSeconds) * time.Second,
		Reason:         request.Reason,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		failHTTP(c, err)
		return
	}
	response.SuccessResponse(
		c,
		apperr.CodeSuccess,
		projectDebugWindow(lease, active),
	)
}

func (handler *HTTPHandler) deactivate(c *gin.Context) {
	if !validHTTPHandler(handler, c) {
		failHTTP(c, ErrInvalidCommand)
		return
	}
	request, err := commonrequest.ParseRequest[deactivateRequest](c)
	if err != nil {
		failHTTP(c, err)
		return
	}
	key, err := parseCanonicalUUID(request.Key)
	if err != nil {
		failHTTP(c, err)
		return
	}
	lease, changed, err := handler.workflow.Deactivate(
		c.Request.Context(),
		key,
		request.Reason,
	)
	if err != nil {
		failHTTP(c, err)
		return
	}
	result := debugWindowProjection{Active: false}
	if changed {
		result = projectDebugWindow(lease, false)
	}
	response.SuccessResponse(c, apperr.CodeSuccess, result)
}

func projectDebugWindow(
	lease commonobservability.DebugLease,
	active bool,
) debugWindowProjection {
	projection := debugWindowProjection{Active: active}
	if lease.Key == "" {
		return projection
	}
	startedAt := lease.StartedAt.UTC()
	expiresAt := lease.ExpiresAt.UTC()
	projection.Key = lease.Key
	projection.Component = lease.Component
	projection.Level = lease.Level
	projection.StartedAt = &startedAt
	projection.ExpiresAt = &expiresAt
	return projection
}

func strictIdempotencyKey(c *gin.Context) (uuid.UUID, error) {
	if c == nil || c.Request == nil {
		return uuid.Nil, ErrInvalidCommand
	}
	values := c.Request.Header.Values(commonrequest.IdempotencyKeyHeader)
	if len(values) != 1 {
		return uuid.Nil, ErrInvalidCommand
	}
	return parseCanonicalUUID(values[0])
}

func parseCanonicalUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return uuid.Nil, ErrInvalidCommand
	}
	return parsed, nil
}

func validHTTPHandler(handler *HTTPHandler, c *gin.Context) bool {
	return handler != nil && handler.workflow != nil &&
		c != nil && c.Request != nil
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Next()
}

func failHTTP(c *gin.Context, err error) {
	if c == nil {
		return
	}
	var applicationError *apperr.AppError
	if errors.As(err, &applicationError) {
		_ = c.Error(err)
		response.ErrorResponse(c, applicationError.Code, err)
		c.Abort()
		return
	}
	switch {
	case errors.Is(err, ErrPermissionDenied):
		err = apperr.New(apperr.CodeForbidden, "permission denied", err)
	case errors.Is(err, ErrRecentAuthenticationRequired):
		err = apperr.New(apperr.CodeForbidden, "recent administrator authentication required", err)
	case errors.Is(err, commonobservability.ErrDebugWindowConflict):
		err = apperr.New(apperr.CodeConflict, "debug window request conflicts with an earlier request", err)
	case errors.Is(err, ErrInvalidCommand),
		errors.Is(err, commonobservability.ErrInvalidDebugWindow):
		err = apperr.New(apperr.CodeBadRequest, "invalid debug window request", err)
	default:
		err = apperr.New(apperr.CodeInternalServer, "debug window unavailable", err)
	}
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}
