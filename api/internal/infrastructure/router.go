package infrastructure

import (
	"net/http"

	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/global"
	cultivationHttp "github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	identityHttp "github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	problemHttp "github.com/huynhanx03/judgify/internal/problem/adapters/driver/http"
)

// RouterGroup contains all handlers and permission checker.
type RouterGroup struct {
	IdentityHandler    *identityHttp.IdentityHandler
	ProblemHandler     *problemHttp.ProblemHandlerGroup
	CultivationHandler *cultivationHttp.CultivationHandlerGroup
	PermChecker        *middlewares.PermissionChecker
}

// NewRouterGroup creates a new RouterGroup.
func NewRouterGroup(
	identityHandler *identityHttp.IdentityHandler,
	problemHandler *problemHttp.ProblemHandlerGroup,
	cultivationHandler *cultivationHttp.CultivationHandlerGroup,
	permChecker *middlewares.PermissionChecker,
) *RouterGroup {
	return &RouterGroup{
		IdentityHandler:    identityHandler,
		ProblemHandler:     problemHandler,
		CultivationHandler: cultivationHandler,
		PermChecker:        permChecker,
	}
}

// registerRoutes registers all routes.
func (rg *RouterGroup) registerRoutes(r *gin.Engine) {
	// Public Routes
	rg.IdentityHandler.RegisterPublic(r.Group("/"))

	// Protected Routes (require authentication)
	protected := r.Group("/")
	protected.Use(middlewares.Authentication(global.Config.JWT.PublicKey))
	{
		rg.IdentityHandler.RegisterProtected(protected, rg.PermChecker)
		rg.ProblemHandler.RegisterProtected(protected, rg.PermChecker)
		rg.CultivationHandler.RegisterProtected(protected, rg.PermChecker)
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
