package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/pkg/judging"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type SubmissionHandlerGroup struct {
	SubmissionHandler SubmissionHandler
	RuntimeCatalog    RuntimeCatalogReader
}

func (h *SubmissionHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	r.GET(
		"/judge/runtimes",
		submissionReadNoStore,
		handler.WrapNoReq(h.readRuntimeCatalog),
	)
}

type RuntimeCatalogReader interface {
	Read(context.Context) (judging.RuntimeCatalog, error)
}

func (h *SubmissionHandlerGroup) readRuntimeCatalog(
	ctx context.Context,
) (judging.RuntimeCatalog, error) {
	if err := ctx.Err(); err != nil {
		return judging.RuntimeCatalog{}, err
	}
	if h == nil || h.RuntimeCatalog == nil {
		return judging.RuntimeCatalog{}, judging.ErrInvalidRuntimeCatalog
	}
	return h.RuntimeCatalog.Read(ctx)
}

func (h *SubmissionHandlerGroup) RegisterProtected(
	r *gin.RouterGroup,
	permissionChecker *middlewares.AuthorizationChecker,
) {
	adminSubmissions := r.Group("/admin/submissions")
	{
		adminSubmissions.POST(
			"/find",
			submissionReadNoStore,
			permissionChecker.RequirePermission(
				permissions.ResourceKeySubmission,
				permissions.PermissionScopeRead,
			),
			handler.Wrap(h.SubmissionHandler.FindAdmin),
		)
		adminSubmissions.GET(
			"/:id",
			submissionReadNoStore,
			permissionChecker.RequirePermission(
				permissions.ResourceKeySubmission,
				permissions.PermissionScopeInspect,
			),
			handler.Wrap(h.SubmissionHandler.GetAdmin),
		)
		adminSubmissions.POST(
			"/:id/rejudge",
			submissionReadNoStore,
			permissionChecker.RequirePermission(
				permissions.ResourceKeySubmission,
				permissions.PermissionScopeUpdate,
			),
			h.SubmissionHandler.Rejudge,
		)
		adminSubmissions.POST(
			"/:id/cancel",
			submissionReadNoStore,
			permissionChecker.RequirePermission(
				permissions.ResourceKeySubmission,
				permissions.PermissionScopeUpdate,
			),
			h.SubmissionHandler.Cancel,
		)
	}

	submissions := r.Group("/submissions")
	{
		submissions.POST("", h.SubmissionHandler.SubmitAccepted)
		submissions.GET("/:id", handler.Wrap(h.SubmissionHandler.Get))
	}

	// List submissions by problem (nested under problems)
	r.GET(
		"/problems/:id/submissions",
		permissionChecker.RequirePermission(
			permissions.ResourceKeySubmission,
			permissions.PermissionScopeRead,
		),
		handler.Wrap(h.SubmissionHandler.FindByProblem),
	)
	r.GET(
		"/problems/:id/my-submissions",
		submissionReadNoStore,
		h.SubmissionHandler.FindMySubmissions,
	)
}

func submissionReadNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Next()
}
