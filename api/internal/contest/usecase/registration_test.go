package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	contestMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newRegistrationTestEnv(t *testing.T) (*registrationService, *contestMocks.MockRegistrationRepository) {
	t.Helper()
	regRepo := contestMocks.NewMockRegistrationRepository(gomock.NewController(t))
	svc := NewRegistrationService(regRepo).(*registrationService)
	return svc, regRepo
}

func TestRegistrationServiceRegisterUsesAtomicRepositoryCommand(t *testing.T) {
	svc, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()
	contestID, userID := testutil.ID(1), testutil.ID(42)

	regRepo.EXPECT().Register(ctx, contestID, userID).Return(nil)

	require.NoError(t, svc.Register(ctx, contestID, userID))
}

func TestRegistrationServiceRegisterMapsDomainFailures(t *testing.T) {
	tests := []struct {
		name        string
		repository  error
		wantCode    int
		wantMessage string
	}{
		{
			name:        "contest not found",
			repository:  entity.ErrContestNotFound,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "contest not found",
		},
		{
			name:        "registration not open",
			repository:  entity.ErrContestRegistrationNotOpen,
			wantCode:    apperr.CodeConflict,
			wantMessage: "contest registration is not open",
		},
		{
			name:        "contest started",
			repository:  entity.ErrContestRegistrationStarted,
			wantCode:    apperr.CodeConflict,
			wantMessage: "contest registration has closed",
		},
		{
			name:        "already registered",
			repository:  entity.ErrContestAlreadyRegistered,
			wantCode:    apperr.CodeConflict,
			wantMessage: "already registered",
		},
		{
			name:        "contest full",
			repository:  entity.ErrContestRegistrationFull,
			wantCode:    apperr.CodeConflict,
			wantMessage: "contest is full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc, regRepo := newRegistrationTestEnv(t)
			ctx := context.Background()
			contestID, userID := testutil.ID(1), testutil.ID(42)
			regRepo.EXPECT().Register(ctx, contestID, userID).Return(test.repository)

			err := svc.Register(ctx, contestID, userID)

			var appErr *apperr.AppError
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, test.wantCode, appErr.Code)
			require.Equal(t, test.wantMessage, appErr.Message)
			require.ErrorIs(t, err, test.repository)
		})
	}
}

func TestRegistrationServiceRegisterPreservesInfrastructureFailure(t *testing.T) {
	svc, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()
	databaseErr := apperr.New(apperr.CodeDatabaseError, "database unavailable", errors.New("driver failed"))
	regRepo.EXPECT().Register(ctx, testutil.ID(1), testutil.ID(42)).Return(databaseErr)

	require.ErrorIs(t, svc.Register(ctx, testutil.ID(1), testutil.ID(42)), databaseErr)
}

func TestRegistrationServiceUnregisterUsesAtomicRepositoryCommand(t *testing.T) {
	svc, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()
	contestID, userID := testutil.ID(1), testutil.ID(42)

	regRepo.EXPECT().Unregister(ctx, contestID, userID).Return(nil)

	require.NoError(t, svc.Unregister(ctx, contestID, userID))
}

func TestRegistrationServiceUnregisterMapsDomainFailures(t *testing.T) {
	tests := []struct {
		name        string
		repository  error
		wantCode    int
		wantMessage string
	}{
		{
			name:        "contest not found",
			repository:  entity.ErrContestNotFound,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "contest not found",
		},
		{
			name:        "registration not open",
			repository:  entity.ErrContestRegistrationNotOpen,
			wantCode:    apperr.CodeConflict,
			wantMessage: "contest registration is not open",
		},
		{
			name:        "contest started",
			repository:  entity.ErrContestRegistrationStarted,
			wantCode:    apperr.CodeConflict,
			wantMessage: "contest registration has closed",
		},
		{
			name:        "registration not found",
			repository:  entity.ErrContestRegistrationNotFound,
			wantCode:    apperr.CodeNotFound,
			wantMessage: "registration not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc, regRepo := newRegistrationTestEnv(t)
			ctx := context.Background()
			contestID, userID := testutil.ID(1), testutil.ID(42)
			regRepo.EXPECT().Unregister(ctx, contestID, userID).Return(test.repository)

			err := svc.Unregister(ctx, contestID, userID)

			var appErr *apperr.AppError
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, test.wantCode, appErr.Code)
			require.Equal(t, test.wantMessage, appErr.Message)
			require.ErrorIs(t, err, test.repository)
		})
	}
}

func TestRegistrationServiceUnregisterPreservesInfrastructureFailure(t *testing.T) {
	svc, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()
	databaseErr := apperr.New(apperr.CodeDatabaseError, "database unavailable", errors.New("driver failed"))
	regRepo.EXPECT().Unregister(ctx, testutil.ID(1), testutil.ID(42)).Return(databaseErr)

	require.ErrorIs(t, svc.Unregister(ctx, testutil.ID(1), testutil.ID(42)), databaseErr)
}

func TestRegistrationServiceIsRegistered(t *testing.T) {
	svc, regRepo := newRegistrationTestEnv(t)
	ctx := context.Background()
	regRepo.EXPECT().Exists(ctx, testutil.ID(1), testutil.ID(42)).Return(true, nil)

	registered, err := svc.IsRegistered(ctx, testutil.ID(1), testutil.ID(42))

	require.NoError(t, err)
	require.True(t, registered)
}
