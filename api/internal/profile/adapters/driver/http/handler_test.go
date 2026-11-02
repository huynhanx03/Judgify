package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/huynhanx03/go-common/pkg/common/apperr"
	"github.com/huynhanx03/go-common/pkg/security/authentication"
	"github.com/stretchr/testify/require"

	identitydto "github.com/huynhanx03/judgify/internal/identity/contract"
)

type profileReaderFunc func(context.Context, uuid.UUID) (*identitydto.ProfileResponse, error)

func (read profileReaderFunc) Get(
	ctx context.Context,
	userID uuid.UUID,
) (*identitydto.ProfileResponse, error) {
	return read(ctx, userID)
}

func TestHandlerReturnsCompatibleAuthenticatedProfileEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	handler, err := NewHandler(profileReaderFunc(func(
		_ context.Context,
		requested uuid.UUID,
	) (*identitydto.ProfileResponse, error) {
		require.Equal(t, userID, requested)
		return &identitydto.ProfileResponse{ID: userID, Username: "judge-user"}, nil
	}))
	require.NoError(t, err)
	router := gin.New()
	router.GET("/users/profile", handler.GetProfile)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/users/profile", nil)
	request = request.WithContext(authentication.WithPrincipal(request.Context(), validPrincipal(userID)))

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			ID       uuid.UUID `json:"id"`
			Username string    `json:"username"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&body))
	require.Equal(t, apperr.CodeSuccess, body.Code)
	require.Equal(t, userID, body.Data.ID)
	require.Equal(t, "judge-user", body.Data.Username)
}

func TestHandlerRejectsMissingPrincipalBeforeReadingProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var calls atomic.Int64
	handler, err := NewHandler(profileReaderFunc(func(
		context.Context,
		uuid.UUID,
	) (*identitydto.ProfileResponse, error) {
		calls.Add(1)
		return nil, nil
	}))
	require.NoError(t, err)
	router := gin.New()
	router.GET("/users/profile", handler.GetProfile)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/profile", nil))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, calls.Load())
}

func validPrincipal(userID uuid.UUID) authentication.Principal {
	now := time.Now().UTC()
	return authentication.Principal{
		Authenticated:      true,
		Subject:            userID.String(),
		UserID:             userID.String(),
		SessionID:          uuid.NewString(),
		TokenID:            uuid.NewString(),
		AuthenticatedAt:    now.Add(-time.Minute),
		AuthenticatedUntil: now.Add(time.Hour),
		SecurityVersion:    1,
	}
}
