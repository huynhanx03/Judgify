package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	contestMocks "github.com/huynhanx03/judgify/internal/contest/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newStandingTestEnv(t *testing.T) (
	*standingService,
	*contestMocks.MockStandingRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	standingRepo := contestMocks.NewMockStandingRepository(ctrl)
	// hub is nil — we don't test SSE broadcast in unit tests
	svc := NewStandingService(standingRepo, nil).(*standingService)
	return svc, standingRepo
}

// --- GetStandings ---

func TestGetStandings_Success(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{
		{ContestID: 1, UserID: 10, SolvedCount: 3, Penalty: 500},
		{ContestID: 1, UserID: 20, SolvedCount: 5, Penalty: 300},
		{ContestID: 1, UserID: 30, SolvedCount: 5, Penalty: 600},
	}, nil)

	resp, err := svc.GetStandings(ctx, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 3)
	// ICPC sort: solved_count DESC, penalty ASC → user20 (5,300) rank 1, user30 (5,600) rank 2, user10 (3,500) rank 3
	assert.Equal(t, 20, resp[0].UserID)
	assert.Equal(t, 1, resp[0].Rank)
	assert.Equal(t, 30, resp[1].UserID)
	assert.Equal(t, 2, resp[1].Rank)
	assert.Equal(t, 10, resp[2].UserID)
	assert.Equal(t, 3, resp[2].Rank)
}

func TestGetStandings_Empty(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().FindByContest(ctx, 1).Return([]*entity.ContestStanding{}, nil)

	resp, err := svc.GetStandings(ctx, 1)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestGetStandings_RepoError(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().FindByContest(ctx, 1).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.GetStandings(ctx, 1)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- UpdateFromVerdict ---

func TestUpdateFromVerdict_NewStandingAccepted(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	// Get returns error → create new standing
	standingRepo.EXPECT().Get(ctx, 1, 42).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	standingRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, st *entity.ContestStanding) error {
			assert.Equal(t, 1, st.SolvedCount)
			assert.Equal(t, 300+0*1200, st.Penalty) // submitTime + waCount*penaltyPerWA
			return nil
		},
	)

	err := svc.UpdateFromVerdict(ctx, 1, 42, 100, true, 300)
	assert.NoError(t, err)
}

func TestUpdateFromVerdict_NewStandingWrongAnswer(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().Get(ctx, 1, 42).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	standingRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, st *entity.ContestStanding) error {
			assert.Equal(t, 0, st.SolvedCount)
			probResult := st.ProblemResults["100"].(map[string]any)
			assert.Equal(t, 1, probResult["wa_count"])
			return nil
		},
	)

	err := svc.UpdateFromVerdict(ctx, 1, 42, 100, false, 0)
	assert.NoError(t, err)
}

func TestUpdateFromVerdict_ExistingStandingAcceptedWithPenalty(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().Get(ctx, 1, 42).Return(&entity.ContestStanding{
		ContestID:   1,
		UserID:      42,
		SolvedCount: 1,
		Penalty:     500,
		ProblemResults: map[string]any{
			"100": map[string]any{"ac": true, "wa_count": 0, "time": 200},
			"200": map[string]any{"ac": false, "wa_count": 3, "time": 0},
		},
	}, nil)

	standingRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, st *entity.ContestStanding) error {
			assert.Equal(t, 2, st.SolvedCount)
			// penalty: 500 (existing) + 400 (submitTime) + 3*1200 (waCount * penaltyPerWA)
			assert.Equal(t, 500+400+3*1200, st.Penalty)
			return nil
		},
	)

	err := svc.UpdateFromVerdict(ctx, 1, 42, 200, true, 400)
	assert.NoError(t, err)
}

func TestUpdateFromVerdict_AlreadyAccepted(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().Get(ctx, 1, 42).Return(&entity.ContestStanding{
		ContestID:   1,
		UserID:      42,
		SolvedCount: 1,
		Penalty:     500,
		ProblemResults: map[string]any{
			"100": map[string]any{"ac": true, "wa_count": 2, "time": 200},
		},
	}, nil)

	standingRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, st *entity.ContestStanding) error {
			// Should NOT increment solved count or change penalty for already-AC problem
			assert.Equal(t, 1, st.SolvedCount)
			assert.Equal(t, 500, st.Penalty)
			return nil
		},
	)

	err := svc.UpdateFromVerdict(ctx, 1, 42, 100, true, 600)
	assert.NoError(t, err)
}

func TestUpdateFromVerdict_UpsertError(t *testing.T) {
	svc, standingRepo := newStandingTestEnv(t)
	ctx := context.Background()

	standingRepo.EXPECT().Get(ctx, 1, 42).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	standingRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.UpdateFromVerdict(ctx, 1, 42, 100, false, 0)
	assert.Error(t, err)
}
