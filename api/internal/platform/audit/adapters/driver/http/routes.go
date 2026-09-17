// Package audithttp exposes the audit query through the product HTTP API.
// It intentionally lives below the transport-neutral audit package so worker
// processes can reuse audit persistence without importing Gin.
package audithttp

import (
	"errors"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

const maxRawQueryBytes = 2048

var ErrInvalidHandler = errors.New("audit http: invalid handler")

var queryKeys = map[string]func(*audit.SearchRequest, string){
	"actor_user_id": func(request *audit.SearchRequest, value string) { request.ActorUserID = value },
	"action":        func(request *audit.SearchRequest, value string) { request.Action = value },
	"resource":      func(request *audit.SearchRequest, value string) { request.Resource = value },
	"resource_id":   func(request *audit.SearchRequest, value string) { request.ResourceID = value },
	"correlation_id": func(request *audit.SearchRequest, value string) {
		request.CorrelationID = value
	},
	"from":   func(request *audit.SearchRequest, value string) { request.From = value },
	"to":     func(request *audit.SearchRequest, value string) { request.To = value },
	"cursor": func(request *audit.SearchRequest, value string) { request.Cursor = value },
	"limit":  func(request *audit.SearchRequest, value string) { request.Limit = value },
}

// Handler exposes one read-only administrative route. Reads deliberately do
// not append another audit row, avoiding an infinite audit-on-audit feed.
type Handler struct {
	reader audit.Searcher
}

func NewHandler(reader audit.Searcher) (*Handler, error) {
	if reader == nil {
		return nil, ErrInvalidHandler
	}
	return &Handler{reader: reader}, nil
}

func (handler *Handler) RegisterProtected(
	router *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	if handler == nil || router == nil || permissionChecker == nil {
		return
	}
	router.GET(
		"/admin/audit",
		noStore,
		permissionChecker.RequirePermission(
			permissions.ResourceKeyAudit,
			permissions.PermissionScopeRead,
		),
		handler.Search,
	)
}

func (handler *Handler) Search(c *gin.Context) {
	if handler == nil || handler.reader == nil || c == nil || c.Request == nil {
		if c != nil {
			fail(c, apperr.New(apperr.CodeInternalServer, "audit unavailable", ErrInvalidHandler))
		}
		return
	}
	request, err := parseQuery(c.Request.URL.RawQuery)
	if err != nil {
		fail(c, apperr.New(apperr.CodeParamInvalid, "invalid audit search", err))
		return
	}
	filter, err := audit.ParseSearchRequest(request)
	if err != nil {
		fail(c, apperr.New(apperr.CodeParamInvalid, "invalid audit search", err))
		return
	}
	page, err := handler.reader.Search(c.Request.Context(), filter)
	if err != nil {
		fail(c, err)
		return
	}
	if page == nil || page.Items == nil {
		fail(c, apperr.New(apperr.CodeInternalServer, "audit unavailable", ErrInvalidHandler))
		return
	}
	response.SuccessResponse(c, apperr.CodeSuccess, page)
}

func parseQuery(raw string) (audit.SearchRequest, error) {
	if len(raw) > maxRawQueryBytes {
		return audit.SearchRequest{}, audit.ErrInvalidSearch
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return audit.SearchRequest{}, audit.ErrInvalidSearch
	}
	request := audit.SearchRequest{}
	for key, candidates := range values {
		assign, allowed := queryKeys[key]
		if !allowed || len(candidates) != 1 || candidates[0] == "" {
			return audit.SearchRequest{}, audit.ErrInvalidSearch
		}
		assign(&request, candidates[0])
	}
	return request, nil
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Next()
}

func fail(c *gin.Context, err error) {
	_ = c.Error(err)
	response.ErrorResponse(c, apperr.CodeInternalServer, err)
	c.Abort()
}
