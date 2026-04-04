package http

import (
	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
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
		problems.GET("/:id", handler.Wrap(h.ProblemHandler.Get))
	}

	tags := r.Group("/tags")
	{
		tags.GET("", handler.Wrap(h.TagHandler.FindAll))
	}

	difficulties := r.Group("/difficulties")
	{
		difficulties.GET("", handler.Wrap(h.DifficultyHandler.FindAll))
	}
}

func (h *ProblemHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
	// Problems
	problems := r.Group("/problems")
	{
		problems.POST("", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeCreate), handler.Wrap(h.ProblemHandler.Create))
		problems.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeUpdate), handler.Wrap(h.ProblemHandler.Update))
		problems.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyProblem, permissions.PermissionScopeDelete), handler.Wrap(h.ProblemHandler.Delete))

		// Test Cases (nested under problems)
		problems.GET("/:id/test-cases", handler.Wrap(h.TestCaseHandler.FindByProblemID))
		problems.POST("/:id/test-cases", permChecker.RequirePermission(permissions.ResourceKeyTestCase, permissions.PermissionScopeCreate), handler.Wrap(h.TestCaseHandler.Create))
	}

	// Test Cases (standalone for update/delete by test case ID)
	testCases := r.Group("/test-cases")
	{
		testCases.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyTestCase, permissions.PermissionScopeUpdate), handler.Wrap(h.TestCaseHandler.Update))
		testCases.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyTestCase, permissions.PermissionScopeDelete), handler.Wrap(h.TestCaseHandler.Delete))
	}

	// Tags
	tags := r.Group("/tags")
	{
		tags.POST("/find", handler.Wrap(h.TagHandler.Find))
		tags.GET("/:id", handler.Wrap(h.TagHandler.Get))
		tags.POST("", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeCreate), handler.Wrap(h.TagHandler.Create))
		tags.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeUpdate), handler.Wrap(h.TagHandler.Update))
		tags.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyTag, permissions.PermissionScopeDelete), handler.Wrap(h.TagHandler.Delete))
	}

	// Difficulties
	difficulties := r.Group("/difficulties")
	{
		difficulties.POST("/find", handler.Wrap(h.DifficultyHandler.Find))
		difficulties.GET("/:id", handler.Wrap(h.DifficultyHandler.Get))
		difficulties.POST("", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeCreate), handler.Wrap(h.DifficultyHandler.Create))
		difficulties.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeUpdate), handler.Wrap(h.DifficultyHandler.Update))
		difficulties.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyDifficulty, permissions.PermissionScopeDelete), handler.Wrap(h.DifficultyHandler.Delete))
	}
}
