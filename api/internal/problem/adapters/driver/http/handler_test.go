package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	commonhandler "github.com/huynhanx03/go-common/pkg/common/http/handler"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/correlation"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/internal/ports"
	problemmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type recordingAuthorizer struct {
	decision authorization.Decision
	request  authorization.Request
	calls    int
}

func (a *recordingAuthorizer) Authorize(_ context.Context, request authorization.Request) (authorization.Decision, error) {
	a.calls++
	a.request = request
	return a.decision, nil
}

type testCaseHandlerStub struct{}

func (testCaseHandlerStub) FindPublicSamples(context.Context, *publicSamplesRequest) ([]*SampleTestCaseResponse, error) {
	return []*SampleTestCaseResponse{}, nil
}

func TestPublicProblemSamplesDoNotRequireAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := &ProblemHandlerGroup{
		ProblemHandler:    NewProblemHandler(nil),
		TestCaseHandler:   testCaseHandlerStub{},
		TagHandler:        NewTagHandler(nil),
		DifficultyHandler: NewDifficultyHandler(nil),
	}
	group.RegisterPublic(router.Group(""))

	request := httptest.NewRequest(http.MethodGet, "/problems/"+uuid.NewString()+"/samples", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
}

func TestCreateProblemSlugConflictReturnsSafeStructured409WithCID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	service := problemmocks.NewMockProblemService(controller)
	userID := uuid.New()
	sessionID := uuid.New()
	cid := "problem-create-conflict"
	service.EXPECT().
		Create(gomock.Any(), userID, gomock.Any()).
		Return(nil, apperr.New(
			apperr.CodeConflict,
			"problem slug is already in use",
			errors.Join(
				ports.ErrProblemSlugConflict,
				errors.New(`duplicate key violates "problems_slug_key"`),
			),
		).WithDetails(map[string]any{
			"field":  "slug",
			"reason": "already_exists",
		}))

	router := gin.New()
	router.POST("/problems", commonhandler.Wrap(NewProblemHandler(service).Create))
	request := httptest.NewRequest(
		http.MethodPost,
		"/problems",
		strings.NewReader(`{
			"slug":"two-sum",
			"title":"Two Sum",
			"description":"Find a pair",
			"difficulty_id":"`+uuid.NewString()+`"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	now := time.Now().UTC()
	principal := authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          sessionID.String(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		SecurityVersion:    1,
		AuthenticatedUntil: now.Add(time.Hour),
	}
	ctx := authentication.WithPrincipal(request.Context(), principal)
	ctx = correlation.WithContext(ctx, cid)
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusConflict, response.Code)
	var body struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
		CID     string         `json:"cid"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, apperr.CodeConflict, body.Code)
	require.Equal(t, "problem slug is already in use", body.Message)
	require.Equal(t, "slug", body.Data["field"])
	require.Equal(t, "already_exists", body.Data["reason"])
	require.Equal(t, cid, body.CID)
	require.NotContains(t, response.Body.String(), "duplicate key")
	require.NotContains(t, response.Body.String(), "problems_slug_key")
}

func TestAuthoringHTTPErrorClassifiesCatalogDriftAndCheckerInput(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		wantCode   int
		wantReason string
	}{
		{
			name:       "authoring catalog version changed",
			cause:      ports.ErrAuthoringCatalogChanged,
			wantCode:   apperr.CodeConflict,
			wantReason: "authoring_catalog_changed",
		},
		{
			name:     "runtime removed from release",
			cause:    ports.ErrRuntimeProfileUnavailable,
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "checker removed from release",
			cause:    ports.ErrCheckerProfileUnavailable,
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "runtime checker tuple removed from release",
			cause:    ports.ErrExecutionCapabilityUnavailable,
			wantCode: apperr.CodeConflict,
		},
		{
			name:     "invalid checker configuration",
			cause:    ports.ErrInvalidCheckerConfiguration,
			wantCode: apperr.CodeBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := errors.Join(errors.New("service context"), tt.cause)
			result := authoringHTTPError(cause)

			var appError *apperr.AppError
			require.ErrorAs(t, result, &appError)
			require.Equal(t, tt.wantCode, appError.Code)
			require.ErrorIs(t, result, tt.cause)
			if tt.wantReason != "" {
				require.Equal(t, map[string]any{
					"reason": tt.wantReason,
				}, appError.Details)
			}
		})
	}
}

func TestLegacyProblemDeleteRouteIsNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	service := problemmocks.NewMockProblemService(controller)
	authorizer := &recordingAuthorizer{
		decision: authorization.Decision{Allowed: true},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID := uuid.NewString()
		principal := authentication.Principal{
			Authenticated:      true,
			Subject:            userID,
			UserID:             userID,
			SessionID:          uuid.NewString(),
			TokenID:            uuid.NewString(),
			AuthenticatedAt:    time.Now().Add(-time.Minute),
			SecurityVersion:    1,
			AuthenticatedUntil: time.Now().Add(time.Hour),
		}
		c.Request = c.Request.WithContext(
			authentication.WithPrincipal(c.Request.Context(), principal),
		)
		c.Next()
	})
	group := &ProblemHandlerGroup{
		ProblemHandler:    NewProblemHandler(service),
		TestCaseHandler:   testCaseHandlerStub{},
		TagHandler:        NewTagHandler(nil),
		DifficultyHandler: NewDifficultyHandler(nil),
	}
	group.RegisterProtected(
		router.Group(""),
		middlewares.NewAuthorizationChecker(authorizer),
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/problems/"+uuid.NewString(),
		nil,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
	require.Zero(t, authorizer.calls)
}

func TestAdminProblemReadsRequireReadCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "find", method: http.MethodPost, path: "/admin/problems/find", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/admin/problems/" + uuid.NewString()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &recordingAuthorizer{decision: authorization.Decision{Allowed: false}}
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
			group := &ProblemHandlerGroup{
				ProblemHandler:    NewProblemHandler(nil),
				TestCaseHandler:   testCaseHandlerStub{},
				TagHandler:        NewTagHandler(nil),
				DifficultyHandler: NewDifficultyHandler(nil),
			}
			group.RegisterProtected(router.Group(""), middlewares.NewAuthorizationChecker(authorizer))

			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, 1, authorizer.calls)
			require.Equal(t, permissions.ResourceKeyProblem, authorizer.request.Resource)
			require.Equal(t, permissions.PermissionScopeRead, authorizer.request.Action)
		})
	}
}

func TestProblemAuthoringCommandsRequireUpdateCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, requestSpec := range []struct {
		path string
		body string
	}{
		{path: "/problems/" + uuid.NewString() + "/drafts", body: `{}`},
		{path: "/problems/" + uuid.NewString() + "/publish", body: `{}`},
		{path: "/problems/" + uuid.NewString() + "/archive", body: `{}`},
	} {
		authorizer := &recordingAuthorizer{decision: authorization.Decision{Allowed: false}}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			principal := authentication.Principal{Authenticated: true, Subject: "user:" + uuid.NewString(), AuthenticatedUntil: time.Now().Add(time.Hour)}
			c.Request = c.Request.WithContext(authentication.WithPrincipal(c.Request.Context(), principal))
			c.Next()
		})
		group := &ProblemHandlerGroup{ProblemHandler: NewProblemHandler(nil), TestCaseHandler: testCaseHandlerStub{}, TagHandler: NewTagHandler(nil), DifficultyHandler: NewDifficultyHandler(nil)}
		group.RegisterProtected(router.Group(""), middlewares.NewAuthorizationChecker(authorizer))
		request := httptest.NewRequest(http.MethodPost, requestSpec.path, strings.NewReader(requestSpec.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
		require.Equal(t, permissions.ResourceKeyProblem, authorizer.request.Resource)
		require.Equal(t, permissions.PermissionScopeUpdate, authorizer.request.Action)
	}
}

func TestAdminTaxonomyReadsRequireReadCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		resource string
	}{
		{name: "find tags", method: http.MethodPost, path: "/tags/find", body: `{}`, resource: permissions.ResourceKeyTag},
		{name: "get tag", method: http.MethodGet, path: "/tags/" + uuid.NewString(), resource: permissions.ResourceKeyTag},
		{name: "find difficulties", method: http.MethodPost, path: "/difficulties/find", body: `{}`, resource: permissions.ResourceKeyDifficulty},
		{name: "get difficulty", method: http.MethodGet, path: "/difficulties/" + uuid.NewString(), resource: permissions.ResourceKeyDifficulty},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &recordingAuthorizer{decision: authorization.Decision{Allowed: false}}
			router := gin.New()
			router.Use(gin.Recovery(), func(c *gin.Context) {
				principal := authentication.Principal{
					Authenticated: true, Subject: "user:" + uuid.NewString(),
					AuthenticatedUntil: time.Now().Add(time.Hour),
				}
				c.Request = c.Request.WithContext(authentication.WithPrincipal(c.Request.Context(), principal))
				c.Next()
			})
			group := &ProblemHandlerGroup{
				ProblemHandler: NewProblemHandler(nil), TestCaseHandler: testCaseHandlerStub{},
				TagHandler: NewTagHandler(nil), DifficultyHandler: NewDifficultyHandler(nil),
			}
			group.RegisterProtected(router.Group(""), middlewares.NewAuthorizationChecker(authorizer))

			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, 1, authorizer.calls)
			require.Equal(t, test.resource, authorizer.request.Resource)
			require.Equal(t, permissions.PermissionScopeRead, authorizer.request.Action)
		})
	}
}
