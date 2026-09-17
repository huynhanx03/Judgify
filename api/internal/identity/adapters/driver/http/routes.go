package http

import (
	"context"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonrequest "github.com/huynhanx03/go-common/pkg/common/http/request"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type OnboardingTraitOfferIssuer func(
	context.Context,
	uuid.UUID,
	uuid.UUID,
) (any, error)

type IdentityHandler struct {
	AuthorizationHandler       AuthorizationHandler
	RoleHandler                RoleHandler
	AttributeDefinitionHandler AttributeDefinitionHandler
	AuthenticationHandler      AuthenticationHandler
	SessionManagementHandler   SessionManagementHandler
	RecoveryContactHandler     RecoveryContactHandler
	UserHandler                UserHandler
	ProfileHandler             ProfileHandler
	ProfileSchemaHandler       ProfileSchemaHandler
	PublicProfileHandler       PublicProfileHandler
	UserInvitation             ports.UserInvitationService
	OnboardingTraitOffer       OnboardingTraitOfferIssuer
	// BrowserSession is the single browser credential and CSRF transport.
	BrowserSession BrowserSessionConfig
}

func (h *IdentityHandler) RegisterPublic(r *gin.RouterGroup) {
	group := r.Group("/auth")
	group.Use(noStore)
	{
		group.POST("/pre-session", h.bootstrapPreSession)
		group.POST("/register", h.registerSession)
		group.POST("/login", h.loginSession)
		// Starting an OAuth flow creates browser-bound state. Keep that state
		// behind the same anonymous pre-session CSRF boundary as every other
		// session-establishing command; a GET must not create it.
		group.POST("/oauth/:provider/start", h.oauthStartSession)
		group.GET("/oauth/:provider/callback", h.oauthCallbackSession)
		group.POST("/oauth/login/inspect", h.oauthInspectLoginSession)
		group.POST("/oauth/login/finalize", h.oauthFinalizeLoginSession)
		group.POST("/forgot-password", h.forgotPassword)
		group.POST("/reset-password", h.resetPassword)
		group.POST("/invitations/activate", h.activateInvitationMutation)
		group.POST("/recovery-contact/verify", h.verifyRecoveryContactMutation)
	}
	r.POST("/onboarding/trait-offers", noStore, h.createOnboardingTraitOffer)
	if h.ProfileSchemaHandler != nil {
		r.GET(
			"/onboarding/profile-schema",
			noStore,
			handler.WrapNoReq(h.ProfileSchemaHandler.GetOnboardingSchema),
		)
	}
}

// RegisterPublicContent mounts identity read models that are safe for
// anonymous product traffic. It stays separate from RegisterPublic so profile
// reads do not share the stricter authentication-command rate-limit boundary.
func (h *IdentityHandler) RegisterPublicContent(r *gin.RouterGroup) {
	if h == nil || r == nil || h.PublicProfileHandler == nil {
		return
	}
	r.GET(
		"/profiles/:username",
		noStore,
		handler.Wrap(h.PublicProfileHandler.GetPublic),
	)
}

func (h *IdentityHandler) createOnboardingTraitOffer(c *gin.Context) {
	if h == nil || h.OnboardingTraitOffer == nil {
		identityHTTPFail(c, apperr.New(
			apperr.CodeInternalServer,
			"onboarding trait offer service unavailable",
			nil,
		))
		return
	}
	preSessionID, err := h.requirePreSessionMutation(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	idempotencyKey, err := parseIdempotencyHeader(c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	offer, err := h.OnboardingTraitOffer(
		c.Request.Context(), preSessionID, idempotencyKey,
	)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	response.Respond(c, offer)
}

func (h *IdentityHandler) activateInvitationMutation(c *gin.Context) {
	if h == nil || h.UserInvitation == nil {
		identityHTTPFail(c, apperr.New(
			apperr.CodeInternalServer,
			"account invitation service unavailable",
			nil,
		))
		return
	}
	if _, err := h.requirePreSessionMutation(c); err != nil {
		identityHTTPFail(c, err)
		return
	}
	request, err := commonrequest.ParseRequest[activateUserInvitationRequest](c)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	result, err := h.UserInvitation.ActivateInvitation(c.Request.Context(), toActivateUserInvitationRequest(request))
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	noStore(c)
	response.Respond(c, toActivateUserInvitationResponse(result))
}

// RegisterRefresh keeps refresh credentials on a purpose-specific boundary.
// It must be mounted behind authentication configured for PurposeRefresh,
// never behind the access-token middleware used by protected application APIs.
func (h *IdentityHandler) RegisterRefresh(r *gin.RouterGroup) {
	r.POST("/auth/refresh", h.refreshSession)
}

func (h *IdentityHandler) RegisterProtected(r *gin.RouterGroup, permChecker *middlewares.AuthorizationChecker) {
	if h == nil || r == nil {
		return
	}
	// This narrow projection is the bootstrapping boundary for a cookie-backed
	// browser. It deliberately exposes neither raw roles nor credentials.
	r.GET("/session/me", noStore, h.sessionMe)
	// Auth
	if h.AuthenticationHandler != nil {
		auth := r.Group("/auth")
		auth.Use(noStore)
		{
			auth.POST("/logout", h.logoutSession)
			auth.POST("/change-password", h.changePasswordSession)
			auth.GET("/oauth/identities", handler.WrapNoReq(h.AuthenticationHandler.OAuthIdentities))
			auth.POST("/oauth/:provider/link/start", h.oauthLinkStartSession)
			auth.POST("/oauth/link/finalize", h.oauthFinalizeLinkSession)
			auth.DELETE("/oauth/identities/:provider", handler.Wrap(h.AuthenticationHandler.OAuthUnlink))
		}
	}
	if h.SessionManagementHandler != nil {
		h.registerSessionManagementRoutes(r)
	}
	if h.RecoveryContactHandler != nil {
		h.registerRecoveryContactRoutes(r)
	}
	// Self-profile routes belong to the authenticated application boundary,
	// not to the administrative permission plane. Keeping them above the nil
	// checker return also makes isolated protected-route composition complete.
	if h.UserHandler != nil {
		users := r.Group("/users")
		users.Use(noStore)
		if h.ProfileHandler != nil {
			users.GET("/profile", h.ProfileHandler.GetProfile)
		}
		users.GET("/profile/attributes", handler.WrapNoReq(h.UserHandler.GetProfileAttributes))
		users.PUT("/profile/attributes", handler.Wrap(h.UserHandler.UpdateProfileAttributes))
	}
	if h.PublicProfileHandler != nil {
		privacy := r.Group("/users/profile/privacy")
		privacy.Use(noStore)
		privacy.GET("", handler.WrapNoReq(h.PublicProfileHandler.GetPrivacy))
		privacy.PUT(
			"",
			captureIdentityCommandID(),
			handler.Wrap(h.PublicProfileHandler.UpdatePrivacy),
		)
	}
	// A nil checker is useful for isolated route tests only. Production
	// composition always installs it before registering admin policy routes.
	if permChecker == nil {
		return
	}

	h.registerAuthorizationRoutes(r, permChecker)
	h.registerRoleRoutes(r, permChecker)

	// Attribute Definitions
	attrDefs := r.Group("/attribute-definitions", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeRead))
	{
		attrDefs.POST("/find", handler.Wrap(h.AttributeDefinitionHandler.Find))
		attrDefs.GET("/:id", handler.Wrap(h.AttributeDefinitionHandler.Get))
		attrDefs.GET("/:id/revisions", handler.Wrap(h.AttributeDefinitionHandler.ListRevisions))
		attrDefs.POST("", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeCreate), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.Create))
		attrDefs.PUT("/:id", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeUpdate), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.Update))
		attrDefs.POST("/:id/activation/preview", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeUpdate), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.PreviewActivation))
		attrDefs.POST("/:id/activation/apply", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeUpdate), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.ApplyActivation))
		attrDefs.POST("/:id/archive/preview", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeDelete), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.PreviewArchive))
		attrDefs.POST("/:id/archive/apply", permChecker.RequirePermission(permissions.ResourceKeyAttributeDefinition, permissions.PermissionScopeDelete), noStore, captureIdentityCommandID(), handler.Wrap(h.AttributeDefinitionHandler.ApplyArchive))
	}

	// Users
	users := r.Group("/users")
	{
		users.POST("/find", permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeRead), handler.Wrap(h.UserHandler.Find))
		users.POST("/invitations/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeCreate),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewInvitation),
		)
		users.POST("/invitations/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeCreate),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplyInvitation),
		)
		users.POST("/:id/invitation/resend/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeCreate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewInvitationResend),
		)
		users.POST("/:id/invitation/resend/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeCreate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplyInvitationResend),
		)
		users.POST("/:id/roles/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewRoles),
		)
		users.POST("/:id/roles/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplyRoles),
		)
		users.POST("/:id/lifecycle/suspend/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewSuspend),
		)
		users.POST("/:id/lifecycle/suspend/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplySuspend),
		)
		users.POST("/:id/lifecycle/reactivate/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewReactivate),
		)
		users.POST("/:id/lifecycle/reactivate/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeUpdate),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplyReactivate),
		)
		users.POST("/:id/lifecycle/delete/preview",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeDelete),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.PreviewDelete),
		)
		users.POST("/:id/lifecycle/delete/apply",
			permChecker.RequirePermission(permissions.ResourceKeyUser, permissions.PermissionScopeDelete),
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.UserHandler.ApplyDelete),
		)
	}
}

type browserSessionMeResponse struct {
	User                  browserSessionUser `json:"user"`
	Session               browserSessionInfo `json:"session"`
	Capabilities          []string           `json:"capabilities"`
	AuthorizationRevision int64              `json:"authorization_revision"`
}

type browserSessionUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type browserSessionInfo struct {
	ID string `json:"id"`
}

func (h *IdentityHandler) sessionMe(c *gin.Context) {
	if h == nil || h.AuthorizationHandler == nil || h.UserHandler == nil {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "session query unavailable", nil))
		return
	}
	ctx := c.Request.Context()
	userID, ok := authctx.UserID(ctx)
	if !ok {
		identityHTTPFail(c, apperr.New(apperr.CodeUnauthorized, "authentication required", nil))
		return
	}
	sessionID, ok := authctx.SessionID(ctx)
	if !ok {
		identityHTTPFail(c, apperr.New(apperr.CodeUnauthorized, "session required", nil))
		return
	}
	profile, err := h.UserHandler.GetProfile(ctx)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if profile == nil || profile.ID != userID.String() {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session profile", nil))
		return
	}
	grants, err := h.AuthorizationHandler.Me(ctx)
	if err != nil {
		identityHTTPFail(c, err)
		return
	}
	if grants == nil || grants.Revision < 0 {
		identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session authorization", nil))
		return
	}
	capabilities := make([]string, 0, len(grants.Capabilities)*2)
	seen := make(map[string]struct{}, len(grants.Capabilities)*2)
	for _, grant := range grants.Capabilities {
		for _, action := range grant.Actions {
			if grant.Resource == "" || action == "" {
				identityHTTPFail(c, apperr.New(apperr.CodeInternalServer, "invalid session authorization", nil))
				return
			}
			value := grant.Resource + ":" + action
			if _, exists := seen[value]; !exists {
				seen[value] = struct{}{}
				capabilities = append(capabilities, value)
			}
		}
	}
	sort.Strings(capabilities)
	response.Respond(c, &browserSessionMeResponse{
		User:         browserSessionUser{ID: profile.ID, Username: profile.Username, FirstName: profile.FirstName, LastName: profile.LastName},
		Session:      browserSessionInfo{ID: sessionID.String()},
		Capabilities: capabilities, AuthorizationRevision: grants.Revision,
	})
}

func (h *IdentityHandler) registerAuthorizationRoutes(
	r *gin.RouterGroup,
	permChecker *middlewares.AuthorizationChecker,
) {
	// The principal projection requires only the protected session boundary.
	// It never returns raw roles, wildcard policies, or internal capabilities.
	authorizationGroup := r.Group("/authorization")
	authorizationGroup.Use(noStore)
	authorizationGroup.GET("/me", handler.WrapNoReq(h.AuthorizationHandler.Me))

	// The policy catalog remains an administrative read surface. Persisted
	// policy rows reference only keys returned by this endpoint.
	authorizationGroup.GET(
		"/catalog",
		permChecker.RequireAnyPermission(
			middlewares.PermissionRequirement{
				Resource: permissions.ResourceKeyRole,
				Action:   permissions.PermissionScopeRead,
			},
			middlewares.PermissionRequirement{
				Resource: permissions.ResourceKeyAuthorization,
				Action:   permissions.PermissionScopeManage,
			},
		),
		handler.WrapNoReq(h.AuthorizationHandler.Catalog),
	)
}

func (h *IdentityHandler) registerRoleRoutes(
	r *gin.RouterGroup,
	permChecker *middlewares.AuthorizationChecker,
) {
	readRoleFoundation := permChecker.RequireAnyPermission(
		middlewares.PermissionRequirement{
			Resource: permissions.ResourceKeyRole,
			Action:   permissions.PermissionScopeRead,
		},
		middlewares.PermissionRequirement{
			Resource: permissions.ResourceKeyAuthorization,
			Action:   permissions.PermissionScopeManage,
		},
	)
	roles := r.Group("/roles", readRoleFoundation)
	{
		roles.GET("", handler.WrapNoReq(h.RoleHandler.FindAll))
		roles.POST("/find", handler.Wrap(h.RoleHandler.Find))
		roles.GET("/:id", handler.Wrap(h.RoleHandler.Get))
		roles.GET("/:id/policies", handler.Wrap(h.RoleHandler.GetPolicies))
		roles.POST("", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeCreate), noStore, captureIdentityCommandID(), handler.Wrap(h.RoleHandler.Create))
		roles.POST("/:id/metadata/preview", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeUpdate), noStore, captureIdentityCommandID(), handler.Wrap(h.RoleHandler.PreviewMetadata))
		roles.POST("/:id/metadata/apply", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeUpdate), noStore, captureIdentityCommandID(), handler.Wrap(h.RoleHandler.ApplyMetadata))
		roles.POST("/:id/archive/preview", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeDelete), noStore, captureIdentityCommandID(), handler.Wrap(h.RoleHandler.PreviewArchive))
		roles.POST("/:id/archive/apply", permChecker.RequirePermission(permissions.ResourceKeyRole, permissions.PermissionScopeDelete), noStore, captureIdentityCommandID(), handler.Wrap(h.RoleHandler.ApplyArchive))
		roles.POST(
			"/:id/policies/preview",
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.RoleHandler.PreviewPolicies),
		)
		roles.POST(
			"/:id/policies/apply",
			permChecker.RequirePermission(permissions.ResourceKeyAuthorization, permissions.PermissionScopeManage),
			noStore,
			captureIdentityCommandID(),
			handler.Wrap(h.RoleHandler.ApplyPolicies),
		)
	}
}

type identityCommandIDContextKey struct{}

func captureIdentityCommandID() gin.HandlerFunc {
	return func(c *gin.Context) {
		commandID, err := commonrequest.ParseCanonicalUUIDIdempotencyKey(
			c.Request.Header,
		)
		if err == nil {
			c.Request = c.Request.WithContext(context.WithValue(
				c.Request.Context(),
				identityCommandIDContextKey{},
				commandID,
			))
		}
		c.Next()
	}
}

func identityCommandID(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	commandID, ok := ctx.Value(identityCommandIDContextKey{}).(uuid.UUID)
	return commandID, ok && commandID != uuid.Nil
}
