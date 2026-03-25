package infrastructure

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/judgify/global"
	cultivationHttp "github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	identityHttp "github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	problemHttp "github.com/huynhanx03/judgify/internal/problem/adapters/driver/http"
	submissionHttp "github.com/huynhanx03/judgify/internal/submission/adapters/driver/http"
	"github.com/huynhanx03/judgify/pkg/algorithm"
	"github.com/huynhanx03/judgify/pkg/common/http/middlewares"
)

// RouterGroup contains all handlers and permission checker.
type RouterGroup struct {
	IdentityHandler    *identityHttp.IdentityHandler
	ProblemHandler     *problemHttp.ProblemHandlerGroup
	CultivationHandler *cultivationHttp.CultivationHandlerGroup
	SubmissionHandler  *submissionHttp.SubmissionHandlerGroup
	PermChecker        *middlewares.PermissionChecker
}

// NewRouterGroup creates a new RouterGroup.
func NewRouterGroup(
	identityHandler *identityHttp.IdentityHandler,
	problemHandler *problemHttp.ProblemHandlerGroup,
	cultivationHandler *cultivationHttp.CultivationHandlerGroup,
	submissionHandler *submissionHttp.SubmissionHandlerGroup,
	permChecker *middlewares.PermissionChecker,
) *RouterGroup {
	return &RouterGroup{
		IdentityHandler:    identityHandler,
		ProblemHandler:     problemHandler,
		CultivationHandler: cultivationHandler,
		SubmissionHandler:  submissionHandler,
		PermChecker:        permChecker,
	}
}

// registerRoutes registers all routes.
func (rg *RouterGroup) registerRoutes(r *gin.Engine) {
	publicAuth := r.Group("/")
	publicAuth.Use(middlewares.RateLimit(middlewares.RateLimitConfig{
		Limit:  global.Config.Server.RateLimit.Limit / 10,
		Burst:  global.Config.Server.RateLimit.Burst / 10,
		Window: time.Duration(global.Config.Server.RateLimit.Window) * time.Second,
	}))
	rg.IdentityHandler.RegisterPublic(publicAuth)

	protected := r.Group("/")
	protected.Use(middlewares.Authentication(global.Config.JWT.PublicKey))
	{
		rg.IdentityHandler.RegisterProtected(protected, rg.PermChecker)
		rg.ProblemHandler.RegisterProtected(protected, rg.PermChecker)
		rg.CultivationHandler.RegisterProtected(protected, rg.PermChecker)
		rg.SubmissionHandler.RegisterProtected(protected)
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
	r.Use(middlewares.RecoveryMiddleware)
	r.Use(middlewares.RequestLogger())
	r.Use(middlewares.CORSMiddleware)

	r.GET("/ping", Ping)

	r.Use(middlewares.RateLimit(middlewares.RateLimitConfig{
		Limit:  global.Config.Server.RateLimit.Limit,
		Burst:  global.Config.Server.RateLimit.Burst,
		Window: time.Duration(global.Config.Server.RateLimit.Window) * time.Second,
	}))

	r.Use(middlewares.CircuitBreakerMiddleware(algorithm.NewCircuitBreaker(
		algorithm.WithFailureThreshold(global.Config.Server.CircuitBreaker.FailureThreshold),
		algorithm.WithSuccessThreshold(global.Config.Server.CircuitBreaker.SuccessThreshold),
		algorithm.WithOpenTimeout(time.Duration(global.Config.Server.CircuitBreaker.OpenTimeout)*time.Second),
	)))

	// Register routes
	routerGroup.registerRoutes(r)

	return r
}
