package infrastructure

import (
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/http/handler"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
	"github.com/huynhanx03/judgify/pkg/permissions"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/global"
	driverHttp "github.com/huynhanx03/judgify/internal/adapters/driver/http"
)

// RouterGroup contains all handlers and permission checker.
type RouterGroup struct {
	RoleHandler                driverHttp.RoleHandler
	PermissionHandler          driverHttp.PermissionHandler
	ResourceHandler            driverHttp.ResourceHandler
	AttributeDefinitionHandler driverHttp.AttributeDefinitionHandler
	AuthenticationHandler      driverHttp.AuthenticationHandler
	UserHandler                driverHttp.UserHandler
	PermChecker                *middlewares.PermissionChecker
}

// NewRouterGroup creates a new RouterGroup.
func NewRouterGroup(
	roleHandler driverHttp.RoleHandler,
	permissionHandler driverHttp.PermissionHandler,
	resourceHandler driverHttp.ResourceHandler,
	attrDefHandler driverHttp.AttributeDefinitionHandler,
	authHandler driverHttp.AuthenticationHandler,
	userHandler driverHttp.UserHandler,
	permChecker *middlewares.PermissionChecker,
) *RouterGroup {
	return &RouterGroup{
		RoleHandler:                roleHandler,
		PermissionHandler:          permissionHandler,
		ResourceHandler:            resourceHandler,
		AttributeDefinitionHandler: attrDefHandler,
		AuthenticationHandler:      authHandler,
		UserHandler:                userHandler,
		PermChecker:                permChecker,
	}
}

// registerRoutes registers all routes.
func (rg *RouterGroup) registerRoutes(r *gin.Engine) {
	// Public Routes
	public := r.Group("/auth")
	{
		public.POST("/register", handler.Wrap(rg.AuthenticationHandler.Register))
		public.POST("/login", handler.Wrap(rg.AuthenticationHandler.Login))
		public.POST("/oauth/:provider/callback", handler.Wrap(rg.AuthenticationHandler.OAuthCallback))
		public.POST("/oauth/:provider/register", handler.Wrap(rg.AuthenticationHandler.OAuthRegister))
		public.POST("/forgot-password", handler.Wrap(rg.AuthenticationHandler.ForgotPassword))
		public.POST("/reset-password", handler.Wrap(rg.AuthenticationHandler.ResetPassword))
	}

	// Protected Routes (require authentication)
	protected := r.Group("/")
	protected.Use(middlewares.Authentication(global.Config.JWT.PublicKey))
	{
		// Auth
		auth := protected.Group("/auth")
		{
			auth.POST("/change-password", handler.Wrap(rg.AuthenticationHandler.ChangePassword))
			auth.POST("/refresh", handler.Wrap(rg.AuthenticationHandler.RefreshToken))
			auth.POST("/oauth/:provider/link", handler.Wrap(rg.AuthenticationHandler.LinkOAuth))
		}

		// Roles - require permission on role resource
		roles := protected.Group("/roles", rg.PermChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeRead))
		{
			roles.POST("/find", handler.Wrap(rg.RoleHandler.Find))
			roles.GET("/:id", handler.Wrap(rg.RoleHandler.Get))
			roles.POST("", rg.PermChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeCreate), handler.Wrap(rg.RoleHandler.Create))
			roles.PUT("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeUpdate), handler.Wrap(rg.RoleHandler.Update))
			roles.DELETE("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeDelete), handler.Wrap(rg.RoleHandler.Delete))
		}

		// Permissions - require permission on permission resource
		perms := protected.Group("/permissions", rg.PermChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeRead))
		{
			perms.POST("/find", handler.Wrap(rg.PermissionHandler.Find))
			perms.GET("/:id", handler.Wrap(rg.PermissionHandler.Get))
			perms.POST("", rg.PermChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeCreate), handler.Wrap(rg.PermissionHandler.Create))
			perms.PUT("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeUpdate), handler.Wrap(rg.PermissionHandler.Update))
			perms.DELETE("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyPermission, permissions.PermissionScopeDelete), handler.Wrap(rg.PermissionHandler.Delete))
		}

		// Resources - require permission on resource resource
		resources := protected.Group("/resources", rg.PermChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeRead))
		{
			resources.POST("/find", handler.Wrap(rg.ResourceHandler.Find))
			resources.GET("/:id", handler.Wrap(rg.ResourceHandler.Get))
			resources.POST("", rg.PermChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeCreate), handler.Wrap(rg.ResourceHandler.Create))
			resources.PUT("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeUpdate), handler.Wrap(rg.ResourceHandler.Update))
			resources.DELETE("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyResource, permissions.PermissionScopeDelete), handler.Wrap(rg.ResourceHandler.Delete))
		}

		// Attribute Definitions - require permission on attribute_definition resource
		attrDefs := protected.Group("/attribute-definitions", rg.PermChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeRead))
		{
			attrDefs.POST("/find", handler.Wrap(rg.AttributeDefinitionHandler.Find))
			attrDefs.GET("/:id", handler.Wrap(rg.AttributeDefinitionHandler.Get))
			attrDefs.POST("", rg.PermChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeCreate), handler.Wrap(rg.AttributeDefinitionHandler.Create))
			attrDefs.PUT("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeUpdate), handler.Wrap(rg.AttributeDefinitionHandler.Update))
			attrDefs.DELETE("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeDelete), handler.Wrap(rg.AttributeDefinitionHandler.Delete))
		}

		// Users
		users := protected.Group("/users")
		{
			users.DELETE("/:id", rg.PermChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeDelete), handler.Wrap(rg.UserHandler.Delete))
			users.PUT("/profile", handler.Wrap(rg.UserHandler.UpdateProfile))
			users.GET("/profile", handler.Wrap(rg.UserHandler.GetProfile))
		}
	}
}

// Ping health check endpoint.
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Judgify service running!",
	})
}

// NewEngine creates and configures the Gin engine.
func NewEngine(routerGroup *RouterGroup) *gin.Engine {
	if global.Config.Server.Mode != "release" {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// Middlewares
	r.Use(middlewares.RecoveryMiddleware)
	r.Use(middlewares.CORSMiddleware)

	r.GET("/ping", Ping)

	// Register routes
	routerGroup.registerRoutes(r)

	return r
}
