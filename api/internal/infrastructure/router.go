package infrastructure

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	commonhealth "github.com/huynhanx03/go-common/pkg/common/health"
	commonhandler "github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/correlation"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	"github.com/huynhanx03/go-common/pkg/security/authentication"

	debuglevel "github.com/huynhanx03/judgify/internal/application/workflow/debuglevel"
	contestHttp "github.com/huynhanx03/judgify/internal/contest/adapters/driver/http"
	cultivationHttp "github.com/huynhanx03/judgify/internal/cultivation/adapters/driver/http"
	identityHttp "github.com/huynhanx03/judgify/internal/identity/adapters/driver/http"
	judgeOperations "github.com/huynhanx03/judgify/internal/judge/operations"
	materialHttp "github.com/huynhanx03/judgify/internal/material/adapters/driver/http"
	notificationHttp "github.com/huynhanx03/judgify/internal/notification/adapters/driver/http"
	audithttp "github.com/huynhanx03/judgify/internal/platform/audit/adapters/driver/http"
	"github.com/huynhanx03/judgify/internal/platform/observability"
	observabilityhttp "github.com/huynhanx03/judgify/internal/platform/observability/adapters/driver/http"
	operationhttp "github.com/huynhanx03/judgify/internal/platform/operation/adapters/driver/http"
	problemHttp "github.com/huynhanx03/judgify/internal/problem/adapters/driver/http"
	realtimeHttp "github.com/huynhanx03/judgify/internal/realtime/adapters/driver/http"
	submissionHttp "github.com/huynhanx03/judgify/internal/submission/adapters/driver/http"
	"github.com/huynhanx03/judgify/pkg/settings"
)

// RouterGroup contains all handlers and authorization checker.
type RouterGroup struct {
	AuditHandler                *audithttp.Handler
	IdentityHandler             *identityHttp.IdentityHandler
	JudgeHandler                *judgeOperations.HTTPHandler
	ProblemHandler              *problemHttp.ProblemHandlerGroup
	CultivationHandler          *cultivationHttp.CultivationHandlerGroup
	SubmissionHandler           *submissionHttp.SubmissionHandlerGroup
	ContestHandler              *contestHttp.ContestHandlerGroup
	MaterialHandler             *materialHttp.MaterialHandlerGroup
	NotificationHandler         *notificationHttp.Handler
	NotificationCampaignHandler *notificationHttp.CampaignHandler
	OperationHandler            *operationhttp.Handler
	RealtimeHandler             *realtimeHttp.HandlerGroup
	DebugLevelHandler           *debuglevel.HTTPHandler
	AuthzChecker                *middlewares.AuthorizationChecker
	AccessVerifier              authentication.Verifier
}

// NewRouterGroup creates a new RouterGroup.
func NewRouterGroup(
	auditHandler *audithttp.Handler,
	identityHandler *identityHttp.IdentityHandler,
	judgeHandler *judgeOperations.HTTPHandler,
	problemHandler *problemHttp.ProblemHandlerGroup,
	cultivationHandler *cultivationHttp.CultivationHandlerGroup,
	submissionHandler *submissionHttp.SubmissionHandlerGroup,
	contestHandler *contestHttp.ContestHandlerGroup,
	materialHandler *materialHttp.MaterialHandlerGroup,
	notificationHandler *notificationHttp.Handler,
	notificationCampaignHandler *notificationHttp.CampaignHandler,
	operationHandler *operationhttp.Handler,
	realtimeHandler *realtimeHttp.HandlerGroup,
	debugLevelHandler *debuglevel.HTTPHandler,
	authzChecker *middlewares.AuthorizationChecker,
	accessVerifier authentication.Verifier,
) *RouterGroup {
	return &RouterGroup{
		AuditHandler:                auditHandler,
		IdentityHandler:             identityHandler,
		JudgeHandler:                judgeHandler,
		ProblemHandler:              problemHandler,
		CultivationHandler:          cultivationHandler,
		SubmissionHandler:           submissionHandler,
		ContestHandler:              contestHandler,
		MaterialHandler:             materialHandler,
		NotificationHandler:         notificationHandler,
		NotificationCampaignHandler: notificationCampaignHandler,
		OperationHandler:            operationHandler,
		RealtimeHandler:             realtimeHandler,
		DebugLevelHandler:           debugLevelHandler,
		AuthzChecker:                authzChecker,
		AccessVerifier:              accessVerifier,
	}
}

// registerRoutes registers all routes.
func (rg *RouterGroup) registerRoutes(
	r *gin.Engine,
	serverConfig settings.Server,
	authenticationConfig settings.Authentication,
) error {
	publicAuthRateLimit, err := buildRateLimitPolicy(serverConfig, 10)
	if err != nil {
		return fmt.Errorf("http router: configure public authentication rate limit: %w", err)
	}
	publicAuthLimiter, err := middlewares.NewRateLimit(publicAuthRateLimit)
	if err != nil {
		return fmt.Errorf("http router: initialize public authentication rate limit: %w", err)
	}
	publicAuth, publicContent := publicRouteBoundaries(
		r,
		publicAuthLimiter,
		rg.AccessVerifier,
		rg.IdentityHandler.BrowserSession.Access.Name,
		authenticationConfig.MaximumCredentialBytes,
	)
	rg.IdentityHandler.RegisterPublic(publicAuth)
	// Refresh is an opaque, single-use HttpOnly cookie flow. It deliberately
	// never passes through JWT bearer authentication.
	rg.IdentityHandler.RegisterRefresh(publicAuth)

	// Public content remains readable anonymously, but a presented access token
	// is verified so personalized projections (for example is_solved and
	// is_registered) are computed from the canonical live principal. Invalid
	// credentials fail closed instead of being silently downgraded to anonymous.
	rg.IdentityHandler.RegisterPublicContent(publicContent)
	rg.ProblemHandler.RegisterPublic(publicContent)
	rg.CultivationHandler.RegisterPublic(publicContent)
	rg.ContestHandler.RegisterPublic(publicContent)
	rg.MaterialHandler.RegisterPublic(publicContent)
	rg.SubmissionHandler.RegisterPublic(publicContent)

	protectedCSRF, err := rg.IdentityHandler.ProtectedCSRFMiddleware()
	if err != nil {
		return fmt.Errorf("http router: configure protected CSRF: %w", err)
	}
	protected := r.Group("/")
	protected.Use(
		middlewares.AuthenticationWithVerifier(
			rg.AccessVerifier,
			middlewares.WithCredentialSources(middlewares.CredentialBearer, middlewares.CredentialCookie),
			middlewares.WithCredentialCookie(rg.IdentityHandler.BrowserSession.Access.Name),
			middlewares.WithMaxCredentialBytes(authenticationConfig.MaximumCredentialBytes),
		),
		requestIdentityLogContext(),
		protectedCSRF,
	)
	{
		protected.POST(
			"/realtime/tickets",
			realtimeHttp.NoStore,
			commonhandler.WrapNoReq(rg.RealtimeHandler.IssueTicket),
		)
		rg.IdentityHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.JudgeHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.AuditHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.ProblemHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.CultivationHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.SubmissionHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.ContestHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.MaterialHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.NotificationHandler.RegisterProtected(protected)
		rg.NotificationCampaignHandler.RegisterProtected(
			protected,
			rg.AuthzChecker,
		)
		rg.OperationHandler.RegisterProtected(protected, rg.AuthzChecker)
		rg.DebugLevelHandler.RegisterProtected(protected, rg.AuthzChecker)
	}
	return nil
}

func buildRateLimitPolicy(config settings.Server, divisor int) (middlewares.RateLimitConfig, error) {
	if divisor <= 0 {
		return middlewares.RateLimitConfig{}, fmt.Errorf("invalid rate-limit divisor")
	}
	policy := middlewares.RateLimitConfig{
		Limit:          dividePositiveCeil(config.RateLimit.Limit, divisor),
		Burst:          dividePositiveCeil(config.RateLimit.Burst, divisor),
		Window:         time.Duration(config.RateLimit.Window) * time.Second,
		TrustedProxies: append([]string(nil), config.TrustedProxies...),
	}
	if _, err := middlewares.NewRateLimiter(policy); err != nil {
		return middlewares.RateLimitConfig{}, err
	}
	return policy, nil
}

func dividePositiveCeil(value, divisor int) int {
	quotient := value / divisor
	if value%divisor != 0 {
		quotient++
	}
	return quotient
}

func publicRouteBoundaries(
	router *gin.Engine,
	authenticationPolicy gin.HandlerFunc,
	verifier authentication.Verifier,
	accessCookieName string,
	maximumCredentialBytes int,
) (*gin.RouterGroup, *gin.RouterGroup) {
	publicAuthentication := router.Group("/")
	publicAuthentication.Use(authenticationPolicy)
	publicContent := publicContentWithOptionalIdentity(router.Group("/"), verifier, accessCookieName, maximumCredentialBytes)
	return publicAuthentication, publicContent
}

func publicContentWithOptionalIdentity(
	parent *gin.RouterGroup,
	verifier authentication.Verifier,
	accessCookieName string,
	maximumCredentialBytes int,
) *gin.RouterGroup {
	group := parent.Group("/")
	group.Use(middlewares.OptionalAuthenticationWithVerifier(
		verifier,
		middlewares.WithCredentialSources(middlewares.CredentialBearer, middlewares.CredentialCookie),
		middlewares.WithCredentialCookie(accessCookieName),
		middlewares.WithMaxCredentialBytes(maximumCredentialBytes),
	))
	group.Use(requestIdentityLogContext())
	return group
}

// requestIdentityLogContext adds stable, non-secret identity references to
// downstream logs after authentication has established the canonical live
// principal. Raw credentials and token identifiers deliberately never become
// ambient fields repeated by every log line.
func requestIdentityLogContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, exists := authentication.FromContext(c.Request.Context())
		if exists && principal.Authenticated {
			fields := make([]commonlogger.Field, 0, 2)
			if principal.UserID != "" {
				fields = append(fields, commonlogger.String("user_id", principal.UserID))
			}
			if principal.SessionID != "" {
				fields = append(fields, commonlogger.String("session_id", principal.SessionID))
			}
			ctx := commonlogger.WithFields(
				c.Request.Context(),
				fields...,
			)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

// Ping health check endpoint.
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Judgify service running!",
	})
}

func registerHealthRoutes(router *gin.Engine, registry *commonhealth.Registry) {
	handler := gin.WrapH(commonhealth.HTTPHandler(registry))
	router.GET("/live", handler)
	router.GET("/ready", handler)
}

const defaultRequestTimeout = 30 * time.Second

func installRequestPolicy(router *gin.Engine, config settings.Server) error {
	if router == nil {
		return fmt.Errorf("http router: nil engine")
	}
	bodyLimit := config.MaxRequestBodyBytes
	if bodyLimit <= 0 {
		bodyLimit = middlewares.DefaultBodyLimit
	}
	requestTimeout := time.Duration(config.RequestTimeoutSeconds) * time.Second
	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}
	deadline, err := middlewares.RequestDeadline(requestTimeout)
	if err != nil {
		return fmt.Errorf("http router: configure request deadline: %w", err)
	}
	router.Use(middlewares.BodyLimit(bodyLimit), deadline)
	return nil
}

// NewEngine creates and configures the Gin engine.
func NewEngine(
	routerGroup *RouterGroup,
	healthRegistry *commonhealth.Registry,
	metrics *observability.Metrics,
	serverConfig settings.Server,
	authenticationConfig settings.Authentication,
	logger *commonlogger.LoggerZap,
) (*gin.Engine, error) {
	if routerGroup == nil || routerGroup.AccessVerifier == nil || routerGroup.RealtimeHandler == nil ||
		routerGroup.NotificationHandler == nil ||
		routerGroup.NotificationCampaignHandler == nil ||
		routerGroup.OperationHandler == nil ||
		routerGroup.DebugLevelHandler == nil ||
		routerGroup.JudgeHandler == nil ||
		routerGroup.AuditHandler == nil || healthRegistry == nil ||
		metrics == nil || logger == nil || logger.Logger == nil {
		return nil, fmt.Errorf("http router: invalid dependencies")
	}
	switch strings.ToLower(strings.TrimSpace(string(serverConfig.Mode))) {
	case "dev", "development", "test":
		gin.SetMode(gin.DebugMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(append([]string(nil), serverConfig.TrustedProxies...)); err != nil {
		return nil, fmt.Errorf("http router: configure trusted proxies: %w", err)
	}
	// Metrics wraps recovery so a recovered panic is still accounted as a 5xx.
	r.Use(GatewayIdentity(os.Getenv("JUDGIFY_GATEWAY_IDENTITY_MODE")))
	r.Use(otelgin.Middleware("judgify-api"))
	r.Use(observabilityhttp.Middleware(metrics, "api"))
	r.Use(middlewares.RecoveryMiddleware)
	r.Use(middlewares.RequestLogger(logger.Logger))
	cors, err := middlewares.CORS(middlewares.CORSOptions{
		AllowedOrigins:   append([]string(nil), authenticationConfig.BrowserOrigins...),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key", "If-Match", identityHttp.CSRFHeaderName},
		ExposedHeaders:   []string{"ETag", "Idempotency-Replayed", "Location", "Retry-After", correlation.Header},
		AllowCredentials: true,
		MaxAge:           10 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("http router: configure CORS: %w", err)
	}
	r.Use(cors)

	r.GET("/ping", Ping)
	registerHealthRoutes(r, healthRegistry)
	// WebSocket owns its own per-IP/per-principal admission, backpressure and
	// origin checks. Register it before request circuit/rate middleware so a
	// long-lived upgraded connection cannot distort ordinary HTTP resilience.
	r.GET("/ws", routerGroup.RealtimeHandler.WebSocket)
	if err := installRequestPolicy(r, serverConfig); err != nil {
		return nil, err
	}

	rateLimitPolicy, err := buildRateLimitPolicy(serverConfig, 1)
	if err != nil {
		return nil, fmt.Errorf("http router: configure rate limit: %w", err)
	}
	rateLimiter, err := middlewares.NewRateLimit(rateLimitPolicy)
	if err != nil {
		return nil, fmt.Errorf("http router: initialize rate limit: %w", err)
	}
	r.Use(rateLimiter)

	// Register routes
	if err := routerGroup.registerRoutes(r, serverConfig, authenticationConfig); err != nil {
		return nil, err
	}

	return r, nil
}
