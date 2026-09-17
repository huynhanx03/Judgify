package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonDTO "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

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

type materialHandlerSpy struct {
	legacyFindCalled bool
	legacyGetCalled  bool
	publicFindCalled bool
	publicGetCalled  bool
	searchCalled     bool
	resolveCalled    bool
}

func (h *materialHandlerSpy) Find(context.Context, *commonDTO.QueryOptions) (*commonDTO.Paginated[*MaterialResponse], error) {
	h.legacyFindCalled = true
	return emptyMaterialPage(), nil
}

func (h *materialHandlerSpy) Get(_ context.Context, request *materialIDRequest) (*MaterialResponse, error) {
	h.legacyGetCalled = true
	return &MaterialResponse{}, nil
}

func (h *materialHandlerSpy) FindPublic(context.Context, *commonDTO.QueryOptions) (*commonDTO.Paginated[*MaterialResponse], error) {
	h.publicFindCalled = true
	return emptyMaterialPage(), nil
}

func (h *materialHandlerSpy) GetPublic(_ context.Context, request *materialIDRequest) (*MaterialResponse, error) {
	h.publicGetCalled = true
	return &MaterialResponse{}, nil
}

func (h *materialHandlerSpy) FindAdmin(context.Context, *commonDTO.QueryOptions) (*commonDTO.Paginated[*MaterialResponse], error) {
	return emptyMaterialPage(), nil
}

func (h *materialHandlerSpy) GetAdmin(_ context.Context, request *materialIDRequest) (*MaterialResponse, error) {
	return &MaterialResponse{}, nil
}

func (*materialHandlerSpy) Create(context.Context, *createMaterialRequest) (*MaterialResponse, error) {
	return nil, nil
}

func (*materialHandlerSpy) Update(context.Context, *updateMaterialRequest) (*MaterialResponse, error) {
	return nil, nil
}

func (*materialHandlerSpy) Delete(context.Context, *materialIDRequest) (*MaterialResponse, error) {
	return nil, nil
}

func (h *materialHandlerSpy) Search(context.Context, *materialSearchRequest) (*materialSearchResponse, error) {
	h.searchCalled = true
	return &materialSearchResponse{Records: []*MaterialResponse{}}, nil
}

func (*materialHandlerSpy) SearchAdmin(context.Context, *adminMaterialSearchRequest) (*adminMaterialSearchResponse, error) {
	return &adminMaterialSearchResponse{Records: []*MaterialResponse{}}, nil
}

func (h *materialHandlerSpy) ResolveSlug(context.Context, *GetMaterialBySlugRequest) (*materialSlugResponse, error) {
	h.resolveCalled = true
	return &materialSlugResponse{}, nil
}

func (*materialHandlerSpy) Related(context.Context, *RelatedMaterialRequest) ([]*MaterialResponse, error) {
	return []*MaterialResponse{}, nil
}

func (*materialHandlerSpy) ListRevisions(context.Context, *ListMaterialRevisionsRequest) (*materialRevisionPage, error) {
	return &materialRevisionPage{Records: []*materialRevisionResponse{}}, nil
}

func (*materialHandlerSpy) GetRevision(context.Context, *GetMaterialRevisionRequest) (*materialRevisionResponse, error) {
	return &materialRevisionResponse{}, nil
}

func (*materialHandlerSpy) RecordView(context.Context, *RecordMaterialViewRequest) (*RecordMaterialViewResponse, error) {
	return &RecordMaterialViewResponse{}, nil
}

func (*materialHandlerSpy) CreateDraft(context.Context, *createMaterialRequest) (*materialMutationReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) SaveDraft(context.Context, *updateMaterialRequest) (*materialMutationReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) PreviewSubmitReview(context.Context, *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	return nil, nil
}
func (*materialHandlerSpy) ApplySubmitReview(context.Context, *archiveMaterialRequest) (*materialLifecycleReceipt, error) {
	return nil, nil
}
func (*materialHandlerSpy) PreviewReturnToDraft(context.Context, *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	return nil, nil
}
func (*materialHandlerSpy) ApplyReturnToDraft(context.Context, *archiveMaterialRequest) (*materialLifecycleReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) PreviewPublish(context.Context, *publishMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	return nil, nil
}

func (*materialHandlerSpy) ApplyPublish(context.Context, *publishMaterialRequest) (*materialLifecycleReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) PreviewArchive(context.Context, *archiveMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	return nil, nil
}

func (*materialHandlerSpy) ApplyArchive(context.Context, *archiveMaterialRequest) (*materialLifecycleReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) PreviewRollback(context.Context, *rollbackMaterialRequest) (*materialLifecyclePreviewResponse, error) {
	return nil, nil
}

func (*materialHandlerSpy) ApplyRollback(context.Context, *rollbackMaterialRequest) (*materialLifecycleReceipt, error) {
	return nil, nil
}

func (*materialHandlerSpy) Rename(context.Context, *renameMaterialRequest) (*materialMutationReceipt, error) {
	return nil, nil
}

func emptyMaterialPage() *commonDTO.Paginated[*MaterialResponse] {
	records := []*MaterialResponse{}
	return &commonDTO.Paginated[*MaterialResponse]{Records: &records}
}

type materialCategoryHandlerStub struct{}

func (materialCategoryHandlerStub) FindAll(context.Context) ([]*MaterialCategoryResponse, error) {
	return []*MaterialCategoryResponse{}, nil
}

func (materialCategoryHandlerStub) Find(context.Context, *commonDTO.QueryOptions) (*commonDTO.Paginated[*MaterialCategoryResponse], error) {
	records := []*MaterialCategoryResponse{}
	return &commonDTO.Paginated[*MaterialCategoryResponse]{Records: &records}, nil
}

func (materialCategoryHandlerStub) Get(_ context.Context, request *materialCategoryIDRequest) (*MaterialCategoryResponse, error) {
	return &MaterialCategoryResponse{ID: request.ID}, nil
}

func (materialCategoryHandlerStub) Create(context.Context, *CreateMaterialCategoryRequest) (*MaterialCategoryResponse, error) {
	return nil, nil
}

func (materialCategoryHandlerStub) Update(context.Context, *UpdateMaterialCategoryRequest) (*MaterialCategoryResponse, error) {
	return nil, nil
}

func (materialCategoryHandlerStub) Delete(context.Context, *materialCategoryIDRequest) (*MaterialCategoryResponse, error) {
	return nil, nil
}

func TestPublicMaterialRoutesUseRestrictedHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spy := &materialHandlerSpy{}
	router := gin.New()
	group := &MaterialHandlerGroup{CategoryHandler: materialCategoryHandlerStub{}, MaterialHandler: spy}
	group.RegisterPublic(router.Group(""))

	findRequest := httptest.NewRequest(http.MethodPost, "/materials/search", strings.NewReader(`{}`))
	findRequest.Header.Set("Content-Type", "application/json")
	findResponse := httptest.NewRecorder()
	router.ServeHTTP(findResponse, findRequest)

	getRequest := httptest.NewRequest(http.MethodGet, "/materials/slug/a-valid-slug", nil)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)

	require.Equal(t, http.StatusOK, findResponse.Code)
	require.Equal(t, http.StatusOK, getResponse.Code)
	require.True(t, spy.searchCalled)
	require.True(t, spy.resolveCalled)
	require.False(t, spy.publicFindCalled)
	require.False(t, spy.publicGetCalled)
	require.False(t, spy.legacyFindCalled)
	require.False(t, spy.legacyGetCalled)
}

func TestProtectedMaterialReadRoutesRequireCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		resource string
	}{
		{name: "material search", method: http.MethodPost, path: "/admin/materials/search", body: `{}`, resource: permissions.ResourceKeyMaterial},
		{name: "material detail", method: http.MethodGet, path: "/admin/materials/" + uuid.NewString(), resource: permissions.ResourceKeyMaterial},
		{name: "material revision history", method: http.MethodGet, path: "/admin/materials/" + uuid.NewString() + "/revisions", resource: permissions.ResourceKeyMaterial},
		{name: "material revision detail", method: http.MethodGet, path: "/admin/materials/" + uuid.NewString() + "/revisions/" + uuid.NewString(), resource: permissions.ResourceKeyMaterial},
		{name: "category find", method: http.MethodPost, path: "/material-categories/find", body: `{}`, resource: permissions.ResourceKeyMaterialCategory},
		{name: "category get", method: http.MethodGet, path: "/material-categories/" + uuid.NewString(), resource: permissions.ResourceKeyMaterialCategory},
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
			group := &MaterialHandlerGroup{
				CategoryHandler: materialCategoryHandlerStub{},
				MaterialHandler: &materialHandlerSpy{},
			}
			group.RegisterProtected(router.Group(""), middlewares.NewAuthorizationChecker(authorizer))

			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, 1, authorizer.calls)
			require.Equal(t, tt.resource, authorizer.request.Resource)
			require.Equal(t, permissions.PermissionScopeRead, authorizer.request.Action)
		})
	}
}

func TestMaterialCommandIDRequiresExactHeaderBodyIdentity(t *testing.T) {
	t.Parallel()
	commandID := uuid.New()
	ctx := context.WithValue(
		context.Background(),
		materialCommandIDContextKey{},
		commandID,
	)
	require.NoError(t, matchingMaterialCommandID(ctx, commandID))
	require.Error(t, matchingMaterialCommandID(ctx, uuid.New()))
	require.Error(t, matchingMaterialCommandID(context.Background(), commandID))
}

func TestMaterialLifecyclePreviewAndApplyUseSameExactCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	materialID := uuid.NewString()
	tests := []struct {
		name   string
		path   string
		action string
	}{
		{name: "publish preview", path: "/admin/materials/" + materialID + "/publish/preview", action: permissions.PermissionScopePublish},
		{name: "publish apply", path: "/admin/materials/" + materialID + "/publish", action: permissions.PermissionScopePublish},
		{name: "archive preview", path: "/admin/materials/" + materialID + "/archive/preview", action: permissions.PermissionScopeArchive},
		{name: "archive apply", path: "/admin/materials/" + materialID + "/archive", action: permissions.PermissionScopeArchive},
		{name: "rollback preview", path: "/admin/materials/" + materialID + "/rollback/preview", action: permissions.PermissionScopeRollback},
		{name: "rollback apply", path: "/admin/materials/" + materialID + "/rollback", action: permissions.PermissionScopeRollback},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &recordingAuthorizer{
				decision: authorization.Decision{Allowed: false},
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				principal := authentication.Principal{
					Authenticated:      true,
					Subject:            uuid.NewString(),
					AuthenticatedUntil: time.Now().Add(time.Hour),
				}
				c.Request = c.Request.WithContext(
					authentication.WithPrincipal(c.Request.Context(), principal),
				)
				c.Next()
			})
			group := &MaterialHandlerGroup{
				CategoryHandler: materialCategoryHandlerStub{},
				MaterialHandler: &materialHandlerSpy{},
			}
			group.RegisterProtected(
				router.Group(""),
				middlewares.NewAuthorizationChecker(authorizer),
			)
			request := httptest.NewRequest(
				http.MethodPost,
				test.path,
				strings.NewReader(`{}`),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, 1, authorizer.calls)
			require.Equal(t, permissions.ResourceKeyMaterial, authorizer.request.Resource)
			require.Equal(t, test.action, authorizer.request.Action)
		})
	}
}
