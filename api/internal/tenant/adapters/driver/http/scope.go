package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/response"

	"github.com/huynhanx03/judgify/internal/authctx"
	internalent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenant"
	"github.com/huynhanx03/judgify/internal/ent/generate/tenantmembership"
	"github.com/huynhanx03/judgify/internal/tenantctx"
)

const TenantIDHeader = "X-Tenant-ID"

// ScopeMiddleware resolves a concrete tenant header against the authenticated
// account's active membership before tenant-bound handlers or authorization.
func ScopeMiddleware(database *internalent.EntClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		resolveScope(c, database, true)
	}
}

// OptionalScopeMiddleware carries a selected tenant through global protected
// routes that also expose tenant-owned profile or policy data. Routes that
// touch tenant-owned entities still fail closed in the Ent interceptor when
// the header is absent.
func OptionalScopeMiddleware(database *internalent.EntClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		resolveScope(c, database, false)
	}
}

func resolveScope(c *gin.Context, database *internalent.EntClient, required bool) {
	userID, ok := authctx.UserID(c.Request.Context())
	if !ok {
		scopeFail(c, apperr.CodeUnauthorized, "unauthenticated")
		return
	}
	rawTenantID := c.GetHeader(TenantIDHeader)
	if rawTenantID == "" {
		if !required {
			c.Next()
			return
		}
		if !AllowsAllScope(c.Request.Method, c.FullPath()) {
			scopeFail(c, apperr.CodeInvalidID, "a concrete tenant is required for this route")
			return
		}
		ids := make([]uuid.UUID, 0)
		err := database.WithPrincipalRead(c.Request.Context(), userID, func(client *generate.Client) error {
			memberships, err := client.TenantMembership.Query().
				Where(tenantmembership.UserIDEQ(userID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).
				WithTenant().All(c.Request.Context())
			if err != nil {
				return err
			}
			ids = make([]uuid.UUID, 0, len(memberships))
			for _, membership := range memberships {
				if membership.Edges.Tenant != nil && membership.Edges.Tenant.Status == tenant.StatusActive {
					ids = append(ids, membership.TenantID)
				}
			}
			return nil
		})
		if err != nil {
			_ = c.Error(err)
			scopeFail(c, apperr.CodeInternalServer, "tenant authorization unavailable")
			return
		}
		scope, err := tenantctx.All(ids)
		if err != nil {
			scopeFail(c, apperr.CodeForbidden, "at least one active tenant membership is required")
			return
		}
		c.Request = c.Request.WithContext(tenantctx.WithContext(c.Request.Context(), scope))
		c.Next()
		return
	}
	tenantID, err := uuid.Parse(rawTenantID)
	if err != nil || tenantID == uuid.Nil || tenantID.String() != rawTenantID {
		scopeFail(c, apperr.CodeInvalidID, "a concrete tenant is required")
		return
	}
	member := false
	err = database.WithPrincipalRead(c.Request.Context(), userID, func(client *generate.Client) error {
		var err error
		member, err = client.TenantMembership.Query().
			Where(tenantmembership.TenantIDEQ(tenantID), tenantmembership.UserIDEQ(userID), tenantmembership.StatusEQ(tenantmembership.StatusActive)).
			Exist(c.Request.Context())
		return err
	})
	if err != nil {
		_ = c.Error(err)
		scopeFail(c, apperr.CodeInternalServer, "tenant authorization unavailable")
		return
	}
	active, err := database.DB(c.Request.Context()).Tenant.Query().
		Where(tenant.IDEQ(tenantID), tenant.StatusEQ(tenant.StatusActive)).
		Exist(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		scopeFail(c, apperr.CodeInternalServer, "tenant authorization unavailable")
		return
	}
	if !member || !active {
		scopeFail(c, apperr.CodeForbidden, "tenant membership required")
		return
	}
	scope, err := tenantctx.Tenant(tenantID)
	if err != nil {
		scopeFail(c, apperr.CodeInvalidID, "a concrete tenant is required")
		return
	}
	c.Request = c.Request.WithContext(tenantctx.WithContext(c.Request.Context(), scope))
	c.Next()
}

// AllowsAllScope is the explicit allowlist for account-scoped tenant lifecycle
// routes and read-only aggregate collections. A method's safety alone never
// widens tenant access to all memberships.
func AllowsAllScope(method, route string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return route == "/tenants" || route == "/tenant-invitations"
	case http.MethodPost:
		switch route {
		case "/tenants", "/tenant-invitations/:invitationID/accept":
			return true
		}
		switch route {
		case "/admin/problems/find", "/tags/find", "/difficulties/find", "/material-categories/find":
			return true
		}
	}
	return false
}

func scopeFail(c *gin.Context, code int, message string) {
	c.Header("Cache-Control", "no-store")
	c.Abort()
	response.ErrorResponse(c, code, apperr.New(code, message, nil))
}
