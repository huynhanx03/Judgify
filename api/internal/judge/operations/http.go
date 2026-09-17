package operations

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

var ErrInvalidHTTPHandler = errors.New("judge operations http: invalid handler")

type Reader interface {
	Get(context.Context) (Overview, error)
}

type HTTPHandler struct {
	reader Reader
}

func NewHTTPHandler(reader Reader) (*HTTPHandler, error) {
	if reader == nil {
		return nil, ErrInvalidHTTPHandler
	}
	return &HTTPHandler{reader: reader}, nil
}

func (value *HTTPHandler) RegisterProtected(
	router *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	if value == nil || value.reader == nil || router == nil || permissionChecker == nil {
		return
	}
	router.GET(
		"/admin/judge/overview",
		judgeOperationsNoStore,
		permissionChecker.RequirePermission(
			permissions.ResourceKeyJudge,
			permissions.PermissionScopeRead,
		),
		handler.WrapNoReq(value.reader.Get),
	)
}

func judgeOperationsNoStore(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")
	ctx.Next()
}
