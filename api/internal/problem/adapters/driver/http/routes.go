package http

import (
	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"

	"github.com/huynhanx03/judgify/pkg/permissions"
)

type ProblemHandlerGroup struct {
	ProblemHandler    ProblemHandler
	TestCaseHandler   TestCaseHandler
	TagHandler        TagHandler
	DifficultyHandler DifficultyHandler
}

// RegisterPublic registers public problem routes (no auth required).
func (h *ProblemHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	problems := r.Group("/problems")
	{
		problems.POST("/find", handler.Wrap(h.ProblemHandler.Find))
		problems.GET("/:id/samples", handler.Wrap(h.TestCaseHandler.FindPublicSamples))
		problems.GET("/:id", handler.Wrap(h.ProblemHandler.Get))
	}

	tags := r.Group("/tags")
	{
		tags.GET("", handler.WrapNoReq(h.TagHandler.FindAll))
	}

	difficulties := r.Group("/difficulties")
	{
		difficulties.GET("", handler.WrapNoReq(h.DifficultyHandler.FindAll))
	}
}

func (h *ProblemHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.AuthorizationChecker) {
	adminProblems := r.Group("/admin/problems")
	{
		adminProblems.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeRead), handler.Wrap(h.ProblemHandler.FindAdmin))
		adminProblems.GET(
			"/authoring/catalog",
			permChecker.RequireAnyPermission(
				middlewares.PermissionRequirement{
					Resource: permissions.ResourceKeyProblem,
					Action:   permissions.PermissionScopeCreate,
				},
				middlewares.PermissionRequirement{
					Resource: permissions.ResourceKeyProblem,
					Action:   permissions.PermissionScopeUpdate,
				},
			),
			handler.WrapNoReq(h.ProblemHandler.GetAuthoringCatalog),
		)
		adminProblems.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeRead), handler.Wrap(h.ProblemHandler.GetAdmin))
		adminProblems.GET("/:id/authoring", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeUpdate), handler.Wrap(h.ProblemHandler.GetAuthoring))
	}

	// Problems
	problems := r.Group("/problems")
	{
		problems.POST("", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeCreate), handler.Wrap(h.ProblemHandler.Create))
		// Testcase content belongs exclusively to an immutable draft revision;
		// there is no independent mutable testcase command surface.
		problems.POST("/:id/drafts", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeUpdate), handler.Wrap(h.ProblemHandler.SaveDraft))
		problems.POST("/:id/publish", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeUpdate), handler.Wrap(h.ProblemHandler.Publish))
		problems.POST("/:id/archive", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeUpdate), handler.Wrap(h.ProblemHandler.Archive))
	}

	// Tags
	tags := r.Group("/tags")
	{
		tags.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeRead), handler.Wrap(h.TagHandler.Find))
		tags.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeRead), handler.Wrap(h.TagHandler.Get))
		tags.POST("", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeCreate), handler.Wrap(h.TagHandler.Create))
		tags.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeUpdate), handler.Wrap(h.TagHandler.Update))
		tags.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeDelete), handler.Wrap(h.TagHandler.Delete))
	}

	// Difficulties
	difficulties := r.Group("/difficulties")
	{
		difficulties.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeRead), handler.Wrap(h.DifficultyHandler.Find))
		difficulties.GET("/:id", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeRead), handler.Wrap(h.DifficultyHandler.Get))
		difficulties.POST("", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeCreate), handler.Wrap(h.DifficultyHandler.Create))
		difficulties.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeUpdate), handler.Wrap(h.DifficultyHandler.Update))
		difficulties.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeDelete), handler.Wrap(h.DifficultyHandler.Delete))
	}
}
