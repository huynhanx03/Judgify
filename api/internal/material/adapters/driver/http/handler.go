package http

import (
	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

// MaterialHandlerGroup contains all material-related handlers.
type MaterialHandlerGroup struct {
	CategoryHandler MaterialCategoryHandler
	MaterialHandler MaterialHandler
}

// RegisterPublic registers public material routes.
func (h *MaterialHandlerGroup) RegisterPublic(r *gin.RouterGroup) {
	categories := r.Group("/material-categories")
	{
		categories.GET("", handler.Wrap(h.CategoryHandler.FindAll))
	}

	materials := r.Group("/materials")
	{
		materials.POST("/find", handler.Wrap(h.MaterialHandler.Find))
		materials.GET("/:id", handler.Wrap(h.MaterialHandler.Get))
	}
}

// RegisterProtected registers protected material routes.
func (h *MaterialHandlerGroup) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
	categories := r.Group("/material-categories")
	{
		categories.POST("", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeCreate), handler.Wrap(h.CategoryHandler.Create))
		categories.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeUpdate), handler.Wrap(h.CategoryHandler.Update))
		categories.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterialCategory, permissions.PermissionScopeDelete), handler.Wrap(h.CategoryHandler.Delete))
	}

	materials := r.Group("/materials")
	{
		materials.POST("", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeCreate), handler.Wrap(h.MaterialHandler.Create))
		materials.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeUpdate), handler.Wrap(h.MaterialHandler.Update))
		materials.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyMaterial, permissions.PermissionScopeDelete), handler.Wrap(h.MaterialHandler.Delete))
	}
}
