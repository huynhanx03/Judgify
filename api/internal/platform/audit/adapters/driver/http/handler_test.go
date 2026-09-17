package audithttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	commonmiddlewares "github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/internal/platform/audit"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type auditReaderStub struct {
	page   *audit.Page
	err    error
	filter audit.Filter
	calls  int
}

func (reader *auditReaderStub) Search(_ context.Context, filter audit.Filter) (*audit.Page, error) {
	reader.calls++
	reader.filter = filter
	return reader.page, reader.err
}

type auditAuthorizerStub struct {
	decision authorization.Decision
	request  authorization.Request
	calls    int
}

func (authorizer *auditAuthorizerStub) Authorize(
	_ context.Context,
	request authorization.Request,
) (authorization.Decision, error) {
	authorizer.calls++
	authorizer.request = request
	return authorizer.decision, nil
}

func TestAuditRouteRequiresReadCapabilityBeforeQuerying(t *testing.T) {
	t.Parallel()

	reader := &auditReaderStub{page: &audit.Page{Items: []audit.Item{}}}
	authorizer := &auditAuthorizerStub{decision: authorization.Decision{Allowed: false}}
	router := newAuditTestRouter(t, reader, authorizer)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authenticatedAuditRequest("/admin/audit"))

	require.Equal(t, http.StatusForbidden, response.Code)
	require.Zero(t, reader.calls)
	require.Equal(t, 1, authorizer.calls)
	require.Equal(t, permissions.ResourceKeyAudit, authorizer.request.Resource)
	require.Equal(t, permissions.PermissionScopeRead, authorizer.request.Action)
}

func TestAuditRouteIsNoStoreAndRejectsUnknownOrDuplicateParameters(t *testing.T) {
	t.Parallel()

	for name, target := range map[string]string{
		"unknown":   "/admin/audit?sort=reason",
		"duplicate": "/admin/audit?action=a&action=b",
	} {
		t.Run(name, func(t *testing.T) {
			reader := &auditReaderStub{page: &audit.Page{Items: []audit.Item{}}}
			authorizer := &auditAuthorizerStub{decision: authorization.Decision{Allowed: true}}
			router := newAuditTestRouter(t, reader, authorizer)

			response := httptest.NewRecorder()
			router.ServeHTTP(response, authenticatedAuditRequest(target))

			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			require.Equal(t, "no-cache", response.Header().Get("Pragma"))
			require.Zero(t, reader.calls)
		})
	}
}

func TestAuditRouteReturnsTypedKeysetPage(t *testing.T) {
	t.Parallel()

	actorID := uuid.MustParse("0198330b-3400-7000-8000-000000000020")
	reader := &auditReaderStub{page: &audit.Page{
		Items: []audit.Item{{
			ID:            uuid.MustParse("0198330b-3400-7000-8000-000000000010"),
			ActorUserID:   &actorID,
			Action:        "identity.role.replace",
			Resource:      "authorization_policy",
			ResourceID:    "role:moderator",
			Reason:        "approved",
			Before:        map[string]any{},
			After:         map[string]any{"revision": json.Number("2")},
			CorrelationID: "018f5fbe-3d77-7e10-8fd1-8b6f1ca32e17",
			OccurredAt:    time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC),
		}},
		NextCursor: "opaque",
		HasMore:    true,
	}}
	authorizer := &auditAuthorizerStub{decision: authorization.Decision{Allowed: true}}
	router := newAuditTestRouter(t, reader, authorizer)
	target := "/admin/audit?actor_user_id=" + actorID.String() + "&action=identity.role.replace&limit=17"

	response := httptest.NewRecorder()
	router.ServeHTTP(response, authenticatedAuditRequest(target))

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, 1, reader.calls)
	require.Equal(t, 17, reader.filter.Limit)
	require.Equal(t, actorID, *reader.filter.ActorUserID)
	require.NotContains(t, response.Body.String(), "actor_session_id")
	require.NotContains(t, response.Body.String(), "created_at")
	var envelope struct {
		Data audit.Page `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.True(t, envelope.Data.HasMore)
	require.Equal(t, "opaque", envelope.Data.NextCursor)
	require.Len(t, envelope.Data.Items, 1)
}

func newAuditTestRouter(
	t *testing.T,
	reader audit.Searcher,
	authorizer authorization.Authorizer,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler, err := NewHandler(reader)
	require.NoError(t, err)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		principal := authentication.Principal{
			Authenticated:      true,
			Subject:            "user:" + uuid.NewString(),
			AuthenticatedUntil: time.Now().Add(time.Hour),
		}
		c.Request = c.Request.WithContext(authentication.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	})
	handler.RegisterProtected(router.Group(""), commonmiddlewares.NewAuthorizationChecker(authorizer))
	return router
}

func authenticatedAuditRequest(target string) *http.Request {
	return httptest.NewRequest(http.MethodGet, target, nil)
}
