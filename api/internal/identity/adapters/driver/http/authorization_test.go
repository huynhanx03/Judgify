package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonmiddlewares "github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"
	"github.com/stretchr/testify/require"

	dto "github.com/huynhanx03/judgify/internal/identity/contract"
)

type authorizationQueryStub struct {
	userID       uuid.UUID
	capabilities *dto.EffectiveCapabilitiesResponse
}

func (stub *authorizationQueryStub) Catalog(context.Context) (*dto.AuthorizationCatalogResponse, error) {
	return &dto.AuthorizationCatalogResponse{Revision: 3}, nil
}

func (stub *authorizationQueryStub) EffectiveCapabilities(
	_ context.Context,
	userID uuid.UUID,
) (*dto.EffectiveCapabilitiesResponse, error) {
	stub.userID = userID
	return stub.capabilities, nil
}

type denyAuthorizationStub struct {
	calls int
}

func (stub *denyAuthorizationStub) Authorize(
	context.Context,
	authorization.Request,
) (authorization.Decision, error) {
	stub.calls++
	return authorization.Decision{Allowed: false, Revision: 3}, nil
}

type authorizationManagerStub struct {
	requests []authorization.Request
}

func (stub *authorizationManagerStub) Authorize(
	_ context.Context,
	request authorization.Request,
) (authorization.Decision, error) {
	stub.requests = append(stub.requests, request)
	return authorization.Decision{
		Allowed: request.Resource == "authorization" && request.Action == "manage",
	}, nil
}

type roleQueryStub struct {
	RoleHandler
}

func (roleQueryStub) FindAll(context.Context) ([]*roleResponse, error) {
	return []*roleResponse{}, nil
}

func TestAuthorizationMeUsesCanonicalAuthenticatedUser(t *testing.T) {
	userID := uuid.New()
	reader := &authorizationQueryStub{capabilities: &dto.EffectiveCapabilitiesResponse{
		Revision: 3,
		Capabilities: []dto.CapabilityGrant{{
			Resource: "problem",
			Actions:  []string{"read"},
		}},
	}}
	handler := NewAuthorizationHandler(reader)

	response, err := handler.Me(authenticatedAuthorizationContext(userID))

	require.NoError(t, err)
	require.Equal(t, userID, reader.userID)
	require.Equal(t, int64(3), response.Revision)
}

func TestAuthorizationMeRejectsMissingPrincipal(t *testing.T) {
	handler := NewAuthorizationHandler(&authorizationQueryStub{})

	_, err := handler.Me(context.Background())

	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeUnauthorized, applicationError.Code)
}

func TestAuthorizationMeRouteRequiresAuthenticationButNoRoleRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	reader := &authorizationQueryStub{capabilities: &dto.EffectiveCapabilitiesResponse{
		Revision:     3,
		Capabilities: []dto.CapabilityGrant{},
	}}
	denier := &denyAuthorizationStub{}
	identity := &IdentityHandler{AuthorizationHandler: NewAuthorizationHandler(reader)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authenticatedAuthorizationContext(userID))
		c.Next()
	})
	identity.registerAuthorizationRoutes(
		router.Group(""),
		commonmiddlewares.NewAuthorizationChecker(denier),
	)

	meRecorder := httptest.NewRecorder()
	router.ServeHTTP(meRecorder, httptest.NewRequest(http.MethodGet, "/authorization/me", nil))
	require.Equal(t, http.StatusOK, meRecorder.Code)
	require.Equal(t, "no-store", meRecorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", meRecorder.Header().Get("Pragma"))
	require.Zero(t, denier.calls)

	catalogRecorder := httptest.NewRecorder()
	router.ServeHTTP(catalogRecorder, httptest.NewRequest(http.MethodGet, "/authorization/catalog", nil))
	require.Equal(t, http.StatusForbidden, catalogRecorder.Code)
	require.Equal(t, "no-store", catalogRecorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", catalogRecorder.Header().Get("Pragma"))
	require.Equal(t, 2, denier.calls)
}

func TestAuthorizationManagerCanReadCatalogAndRoleFoundation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	reader := &authorizationQueryStub{}
	manager := &authorizationManagerStub{}
	identity := &IdentityHandler{
		AuthorizationHandler: NewAuthorizationHandler(reader),
		RoleHandler:          roleQueryStub{},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authenticatedAuthorizationContext(userID))
		c.Next()
	})
	checker := commonmiddlewares.NewAuthorizationChecker(manager)
	identity.registerAuthorizationRoutes(router.Group(""), checker)
	identity.registerRoleRoutes(router.Group(""), checker)

	for _, route := range []string{"/authorization/catalog", "/roles"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		require.Equal(t, http.StatusOK, recorder.Code, route)
	}
	require.Len(t, manager.requests, 4)
	require.Equal(t, "role", manager.requests[0].Resource)
	require.Equal(t, "authorization", manager.requests[1].Resource)
	require.Equal(t, "role", manager.requests[2].Resource)
	require.Equal(t, "authorization", manager.requests[3].Resource)
}

func authenticatedAuthorizationContext(userID uuid.UUID) context.Context {
	now := time.Now().UTC()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          uuid.NewString(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		AuthenticatedUntil: now.Add(time.Hour),
		SecurityVersion:    1,
	}
	return authentication.WithPrincipal(context.Background(), principal)
}
