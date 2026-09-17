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
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/common/http/middlewares"
	commonDTO "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/huynhanx03/go-common/pkg/security/authorization"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	"github.com/huynhanx03/judgify/internal/testutil"
	"github.com/huynhanx03/judgify/internal/testutil/mocks"
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

func TestAnonymousContestRoutesUseRestrictedPublicReads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	service := mocks.NewMockContestService(ctrl)
	contestID := testutil.ID(1)
	records := []*contract.ContestResponse{}
	service.EXPECT().FindPublic(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query *commonDTO.QueryOptions) (*commonDTO.Paginated[*contract.ContestResponse], error) {
			require.Len(t, query.Filters, 1)
			require.Equal(t, "status", query.Filters[0].Key)
			require.Equal(t, constant.ContestStatusDraft, query.Filters[0].Value)
			return &commonDTO.Paginated[*contract.ContestResponse]{Records: &records}, nil
		},
	)
	service.EXPECT().GetPublic(gomock.Any(), contestID, uuid.Nil).
		Return(nil, apperr.New(apperr.CodeNotFound, "contest not found", nil))

	router := gin.New()
	group := &ContestHandlerGroup{
		ContestHandler:      NewContestHandler(service),
		RegistrationHandler: NewRegistrationHandler(nil),
		StandingHandler:     NewStandingHandler(nil),
		RatingHandler:       NewRatingHandler(nil),
		ContentHandler:      NewContentHandler(nil),
	}
	group.RegisterPublic(router.Group(""))

	findRequest := httptest.NewRequest(http.MethodPost, "/contests/find", strings.NewReader(
		`{"filters":[{"key":"status","value":"draft","type":"exact"}]}`,
	))
	findRequest.Header.Set("Content-Type", "application/json")
	findResponse := httptest.NewRecorder()
	router.ServeHTTP(findResponse, findRequest)
	require.Equal(t, http.StatusOK, findResponse.Code)

	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/contests/"+contestID.String(), nil))
	require.Equal(t, http.StatusNotFound, getResponse.Code)
}

func TestAdminContestReadsRequireReadCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "find", method: http.MethodPost, path: "/admin/contests/find", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/admin/contests/" + uuid.NewString()},
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
			group := &ContestHandlerGroup{
				ContestHandler:      NewContestHandler(nil),
				RegistrationHandler: NewRegistrationHandler(nil),
				StandingHandler:     NewStandingHandler(nil),
				RatingHandler:       NewRatingHandler(nil),
				ContentHandler:      NewContentHandler(nil),
			}
			group.RegisterProtected(router.Group(""), middlewares.NewAuthorizationChecker(authorizer))

			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, 1, authorizer.calls)
			require.Equal(t, permissions.ResourceKeyContest, authorizer.request.Resource)
			require.Equal(t, permissions.PermissionScopeRead, authorizer.request.Action)
		})
	}
}
