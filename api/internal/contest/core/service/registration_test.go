package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/contest/constant"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	contestMocks "github.com/huynhanx03/judgify/internal/contest/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newRegistrationTestEnv(t *testing.T) (
	*registrationService,
	*contestMocks.MockContestRepository,
	*contestMocks.MockRegistrationRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	contestRepo := contestMocks.NewMockContestRepository(ctrl)
	regRepo := contestMocks.NewMockRegistrationRepository(ctrl)

	svc := NewRegistrationService(regRepo, contestRepo).(*registrationService)
	return svc, contestRepo, regRepo
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, nil)
	regRepo.EXPECT().Create(ctx, 1, 42).Return(nil)

	err := svc.Register(ctx, 1, 42)
	assert.NoError(t, err)
}

func TestRegister_ContestNotFound(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "contest not found", nil))

	err := svc.Register(ctx, 999, 42)
	assert.Error(t, err)
}

func TestRegister_ContestRunning(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusRunning,
	}, nil)

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}

func TestRegister_ContestEnded(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusEnded,
	}, nil)

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}

func TestRegister_ContestFull(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:             1,
		Status:         constant.StatusUpcoming,
		MaxParticipants: 10,
	}, nil)

	regRepo.EXPECT().CountByContest(ctx, 1).Return(10, nil)

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeConflict, appErr.Code)
}

func TestRegister_AlreadyRegistered(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(true, nil)

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeConflict, appErr.Code)
}

func TestRegister_CountError(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:             1,
		Status:         constant.StatusDraft,
		MaxParticipants: 10,
	}, nil)

	regRepo.EXPECT().CountByContest(ctx, 1).Return(0, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)
}

func TestRegister_ExistsCheckError(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)
}

func TestRegister_CreateError(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, nil)
	regRepo.EXPECT().Create(ctx, 1, 42).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Register(ctx, 1, 42)
	assert.Error(t, err)
}

// --- Unregister ---

func TestUnregister_Success(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(true, nil)
	regRepo.EXPECT().Delete(ctx, 1, 42).Return(nil)

	err := svc.Unregister(ctx, 1, 42)
	assert.NoError(t, err)
}

func TestUnregister_ContestNotFound(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "contest not found", nil))

	err := svc.Unregister(ctx, 999, 42)
	assert.Error(t, err)
}

func TestUnregister_ContestRunning(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusRunning,
	}, nil)

	err := svc.Unregister(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}

func TestUnregister_ContestEnded(t *testing.T) {
	svc, contestRepo, _ := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusEnded,
	}, nil)

	err := svc.Unregister(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}

func TestUnregister_NotRegistered(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusDraft,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, nil)

	err := svc.Unregister(ctx, 1, 42)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
}

func TestUnregister_DeleteError(t *testing.T) {
	svc, contestRepo, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Status: constant.StatusUpcoming,
	}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(true, nil)
	regRepo.EXPECT().Delete(ctx, 1, 42).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Unregister(ctx, 1, 42)
	assert.Error(t, err)
}

// --- IsRegistered ---

func TestIsRegistered_True(t *testing.T) {
	svc, _, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(true, nil)

	result, err := svc.IsRegistered(ctx, 1, 42)
	assert.NoError(t, err)
	assert.True(t, result)
}

func TestIsRegistered_False(t *testing.T) {
	svc, _, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, nil)

	result, err := svc.IsRegistered(ctx, 1, 42)
	assert.NoError(t, err)
	assert.False(t, result)
}

func TestIsRegistered_RepoError(t *testing.T) {
	svc, _, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	result, err := svc.IsRegistered(ctx, 1, 42)
	assert.Error(t, err)
	assert.False(t, result)
}
