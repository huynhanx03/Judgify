package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/pkg/judging"
	"github.com/huynhanx03/judgify/pkg/permissions"
)

type submissionAuthorizer struct {
	request authorization.Request
	calls   int
}

func (authorizer *submissionAuthorizer) Authorize(
	_ context.Context,
	request authorization.Request,
) (authorization.Decision, error) {
	authorizer.calls++
	authorizer.request = request
	return authorization.Decision{Allowed: false}, nil
}

type runtimeCatalogReaderStub struct {
	catalog judging.RuntimeCatalog
	err     error
}

func (stub runtimeCatalogReaderStub) Read(
	context.Context,
) (judging.RuntimeCatalog, error) {
	return stub.catalog, stub.err
}

func TestRuntimeCatalogIsPublicAndMatchesJudgePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	catalog, err := judging.NewRuntimeCatalog([]judging.RuntimeIdentity{
		{
			Key:         "cpp17",
			Language:    "cpp",
			DisplayName: "C++17",
		},
	})
	if err != nil {
		t.Fatalf("build runtime catalog fixture: %v", err)
	}
	group := &SubmissionHandlerGroup{
		RuntimeCatalog: runtimeCatalogReaderStub{catalog: catalog},
	}
	group.RegisterPublic(router.Group(""))

	request := httptest.NewRequest(http.MethodGet, "/judge/runtimes", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("runtime catalog cache headers = %#v", response.Header())
	}
	var envelope struct {
		Data judging.RuntimeCatalog `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Version != judging.RuntimeCatalogVersion ||
		envelope.Data.MaximumSourceCodeBytes != judging.MaximumSourceCodeBytes ||
		len(envelope.Data.Runtimes) != 1 ||
		envelope.Data.Runtimes[0].Key != "cpp17" {
		t.Fatalf("unexpected runtime catalog: %+v", envelope.Data)
	}
}

func TestAllProblemSubmissionsRequireModerationReadCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authorizer := &submissionAuthorizer{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		principal := authentication.Principal{
			Authenticated:      true,
			Subject:            "user:" + uuid.NewString(),
			AuthenticatedUntil: time.Now().Add(time.Hour),
		}
		c.Request = c.Request.WithContext(
			authentication.WithPrincipal(c.Request.Context(), principal),
		)
		c.Next()
	})
	group := &SubmissionHandlerGroup{
		SubmissionHandler: NewSubmissionHandler(nil),
	}
	group.RegisterProtected(
		router.Group(""),
		middlewares.NewAuthorizationChecker(authorizer),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/problems/"+uuid.NewString()+"/submissions",
		nil,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if authorizer.calls != 1 ||
		authorizer.request.Resource != permissions.ResourceKeySubmission ||
		authorizer.request.Action != permissions.PermissionScopeRead {
		t.Fatalf("authorization request = %+v, calls = %d", authorizer.request, authorizer.calls)
	}
}

func TestAdminSubmissionMonitoringRoutesRequireLeastPrivilegeCapabilities(t *testing.T) {
	for _, test := range []struct {
		name       string
		method     string
		path       string
		wantAction string
	}{
		{
			name: "safe list", method: http.MethodPost,
			path: "/admin/submissions/find", wantAction: permissions.PermissionScopeRead,
		},
		{
			name: "source and diagnostics detail", method: http.MethodGet,
			path: "/admin/submissions/" + uuid.NewString(), wantAction: permissions.PermissionScopeInspect,
		},
		{
			name: "rejudge mutation", method: http.MethodPost,
			path: "/admin/submissions/" + uuid.NewString() + "/rejudge", wantAction: permissions.PermissionScopeUpdate,
		},
		{
			name: "cancel mutation", method: http.MethodPost,
			path: "/admin/submissions/" + uuid.NewString() + "/cancel", wantAction: permissions.PermissionScopeUpdate,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			authorizer := &submissionAuthorizer{}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				principal := authentication.Principal{
					Authenticated:      true,
					Subject:            "user:" + uuid.NewString(),
					AuthenticatedUntil: time.Now().Add(time.Hour),
				}
				c.Request = c.Request.WithContext(
					authentication.WithPrincipal(c.Request.Context(), principal),
				)
				c.Next()
			})
			group := &SubmissionHandlerGroup{
				SubmissionHandler: NewSubmissionHandler(nil),
			}
			group.RegisterProtected(
				router.Group(""),
				middlewares.NewAuthorizationChecker(authorizer),
			)

			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if authorizer.calls != 1 ||
				authorizer.request.Resource != permissions.ResourceKeySubmission ||
				authorizer.request.Action != test.wantAction {
				t.Fatalf("authorization request = %+v, calls = %d", authorizer.request, authorizer.calls)
			}
		})
	}
}

func TestAdminSubmissionResponsesCannotBeStored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/submissions", submissionReadNoStore, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/admin/submissions", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Header().Get("Cache-Control") != "no-store" ||
		response.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers = %#v", response.Header())
	}
}
