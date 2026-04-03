package http

import (
	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type IdentityHandler struct {
	RoleHandler                RoleHandler
	PermissionHandler          PermissionHandler
	ResourceHandler            ResourceHandler
	AttributeDefinitionHandler AttributeDefinitionHandler
	AuthenticationHandler      AuthenticationHandler
	UserHandler                UserHandler
	ProfileHandler             ProfileHandler
}

func (h *IdentityHandler) RegisterPublic(r *gin.RouterGroup) {
	group := r.Group("/auth")
	{
		group.POST("/register", handler.Wrap(h.AuthenticationHandler.Register))
		group.POST("/login", handler.Wrap(h.AuthenticationHandler.Login))
		group.POST("/oauth/:provider/callback", handler.Wrap(h.AuthenticationHandler.OAuthCallback))
		group.POST("/oauth/:provider/register", handler.Wrap(h.AuthenticationHandler.OAuthRegister))
		group.POST("/forgot-password", handler.Wrap(h.AuthenticationHandler.ForgotPassword))
		group.POST("/reset-password", handler.Wrap(h.AuthenticationHandler.ResetPassword))
	}
}

func (h *IdentityHandler) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.PermissionChecker) {
	// Auth
	auth := r.Group("/auth")
	{
		auth.POST("/change-password", handler.Wrap(h.AuthenticationHandler.ChangePassword))
		auth.POST("/refresh", handler.Wrap(h.AuthenticationHandler.RefreshToken))
		auth.POST("/oauth/:provider/link", handler.Wrap(h.AuthenticationHandler.LinkOAuth))
	}

	// Roles
	roles := r.Group("/roles", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeRead))
	{
		roles.GET("", handler.Wrap(h.RoleHandler.FindAll))
		roles.POST("/find", handler.Wrap(h.RoleHandler.Find))
		roles.GET("/:id", handler.Wrap(h.RoleHandler.Get))
		roles.POST("", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeCreate), handler.Wrap(h.RoleHandler.Create))
		roles.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeUpdate), handler.Wrap(h.RoleHandler.Update))
		roles.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeDelete), handler.Wrap(h.RoleHandler.Delete))
	}

	// Permissions
	perms := r.Group("/permissions", permChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeRead))
	{
		perms.GET("", handler.Wrap(h.PermissionHandler.FindAll))
		perms.POST("/find", handler.Wrap(h.PermissionHandler.Find))
		perms.GET("/:id", handler.Wrap(h.PermissionHandler.Get))
		perms.POST("", permChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeCreate), handler.Wrap(h.PermissionHandler.Create))
		perms.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeUpdate), handler.Wrap(h.PermissionHandler.Update))
		perms.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeDelete), handler.Wrap(h.PermissionHandler.Delete))
	}

	// Resources
	resources := r.Group("/resources", permChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeRead))
	{
		resources.GET("", handler.Wrap(h.ResourceHandler.FindAll))
		resources.POST("/find", handler.Wrap(h.ResourceHandler.Find))
		resources.GET("/:id", handler.Wrap(h.ResourceHandler.Get))
		resources.POST("", permChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeCreate), handler.Wrap(h.ResourceHandler.Create))
		resources.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeUpdate), handler.Wrap(h.ResourceHandler.Update))
		resources.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeDelete), handler.Wrap(h.ResourceHandler.Delete))
	}

	// Attribute Definitions
	attrDefs := r.Group("/attribute-definitions", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeRead))
	{
		attrDefs.POST("/find", handler.Wrap(h.AttributeDefinitionHandler.Find))
		attrDefs.GET("/:id", handler.Wrap(h.AttributeDefinitionHandler.Get))
		attrDefs.POST("", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeCreate), handler.Wrap(h.AttributeDefinitionHandler.Create))
		attrDefs.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeUpdate), handler.Wrap(h.AttributeDefinitionHandler.Update))
		attrDefs.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeDelete), handler.Wrap(h.AttributeDefinitionHandler.Delete))
	}

	// Users
	users := r.Group("/users")
	{
		users.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeRead), handler.Wrap(h.UserHandler.Find))
		users.POST("", permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeCreate), handler.Wrap(h.UserHandler.Create))
		users.PATCH("/:id", permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate), handler.Wrap(h.UserHandler.Update))
		users.DELETE("/:id", permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeDelete), handler.Wrap(h.UserHandler.Delete))
		users.PUT("/profile", handler.Wrap(h.UserHandler.UpdateProfile))
		users.GET("/profile",
			middlewares.Parallel(
				h.ProfileHandler.WithBasicProfile,
				h.ProfileHandler.WithCultivationStats,
				h.ProfileHandler.WithUserTraits,
				h.ProfileHandler.WithElementExps,
				h.ProfileHandler.WithLevelsRanks,
				h.ProfileHandler.WithProblemStats,
			),
			h.ProfileHandler.GetProfile,
		)
	}
}
