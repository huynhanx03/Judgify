package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	contestmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newStandingTestEnv(t *testing.T) (
	*standingService,
	*contestmocks.MockStandingRepository,
	*contestmocks.MockContestRepository,
) {
	t.Helper()
	controller := gomock.NewController(t)
	standingRepository := contestmocks.NewMockStandingRepository(controller)
	contestRepository := contestmocks.NewMockContestRepository(controller)
	service := NewStandingService(
		standingRepository,
		contestRepository,
	).(*standingService)
	return service, standingRepository, contestRepository
}

func TestGetStandingsUsesPublicBoundaryAndCanonicalRanking(t *testing.T) {
	service, standings, contests := newStandingTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	contests.EXPECT().GetPublic(ctx, contestID).Return(&entity.Contest{
		ID:                         contestID,
		State:                      entity.ContestRunning,
		StandingsProjectionVersion: 4,
	}, nil)
	standings.EXPECT().FindByContest(ctx, contestID).Return([]*entity.ContestStanding{
		{ContestID: contestID, UserID: testutil.ID(10), Username: "carol", SolvedCount: 3, Penalty: 500},
		{ContestID: contestID, UserID: testutil.ID(20), Username: "alice", SolvedCount: 5, Penalty: 300},
		{ContestID: contestID, UserID: testutil.ID(30), Username: "bob", SolvedCount: 5, Penalty: 600},
	}, nil)

	response, err := service.GetStandings(ctx, contestID)

	require.NoError(t, err)
	require.Equal(t, contestID, response.ContestID)
	require.Equal(t, "official", response.View)
	require.Equal(t, int64(4), response.ProjectionVersion)
	require.Equal(t, "4", response.LastCursor)
	require.Len(t, response.Items, 3)
	require.Equal(t, testutil.ID(20), response.Items[0].UserID)
	require.Equal(t, 1, response.Items[0].Rank)
	require.Equal(t, testutil.ID(30), response.Items[1].UserID)
	require.Equal(t, 2, response.Items[1].Rank)
	require.Equal(t, testutil.ID(10), response.Items[2].UserID)
	require.Equal(t, 3, response.Items[2].Rank)
}

func TestGetStandingsReturnsFrozenSnapshotOnlyDuringFreezeWindow(t *testing.T) {
	service, standings, contests := newStandingTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	now := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	freezeAt := now.Add(-time.Minute)
	service.clock = func() time.Time { return now }
	problemID := testutil.ID(100).String()
	contests.EXPECT().GetPublic(ctx, contestID).Return(&entity.Contest{
		ID:                         contestID,
		State:                      entity.ContestRunning,
		EndTime:                    now.Add(time.Hour),
		FreezeStandingsAt:          &freezeAt,
		StandingsProjectionVersion: 9,
	}, nil)
	standings.EXPECT().FindByContest(ctx, contestID).Return([]*entity.ContestStanding{{
		ID:                        testutil.ID(50),
		ContestID:                 contestID,
		UserID:                    testutil.ID(10),
		Username:                  "alice",
		SolvedCount:               5,
		Penalty:                   900,
		FrozenSolvedCount:         3,
		FrozenPenalty:             420,
		FrozenVersion:             7,
		FrozenProblemResults:      map[string]any{problemID: map[string]any{"ac": true}},
		FrozenPendingAttemptCount: 2,
		FrozenPendingByProblem:    map[string]int{problemID: 2},
		VisiblePendingAttempts:    99,
		VisiblePendingByProblem:   map[string]int{problemID: 99},
	}}, nil)

	response, err := service.GetStandings(ctx, contestID)

	require.NoError(t, err)
	require.Equal(t, "frozen", response.View)
	require.NotNil(t, response.FrozenAt)
	require.Equal(t, freezeAt, *response.FrozenAt)
	require.Len(t, response.Items, 1)
	require.Equal(t, 3, response.Items[0].SolvedCount)
	require.Equal(t, 420, response.Items[0].Penalty)
	require.Equal(t, 2, response.Items[0].PendingAttempts)
	require.Equal(t, map[string]int{problemID: 2}, response.Items[0].PendingByProblem)
}

func TestGetStandingsDoesNotExposeDraftContest(t *testing.T) {
	service, _, contests := newStandingTestEnv(t)
	contestID := testutil.ID(1)
	notFound := errors.New("contest not found")
	contests.EXPECT().GetPublic(gomock.Any(), contestID).Return(nil, notFound)

	response, err := service.GetStandings(context.Background(), contestID)

	require.Nil(t, response)
	require.ErrorIs(t, err, notFound)
}

func TestGetOfficialStandingsUsesAdminContestBoundary(t *testing.T) {
	service, standings, contests := newStandingTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	contests.EXPECT().Get(ctx, contestID).Return(&entity.Contest{
		ID: contestID, State: entity.ContestDraft, StandingsProjectionVersion: 2,
	}, nil)
	standings.EXPECT().FindByContest(ctx, contestID).Return([]*entity.ContestStanding{{
		ContestID: contestID, UserID: testutil.ID(10), SolvedCount: 1,
	}}, nil)

	response, err := service.GetOfficialStandings(ctx, contestID)

	require.NoError(t, err)
	require.Equal(t, "official", response.View)
	require.Equal(t, int64(2), response.ProjectionVersion)
	require.Len(t, response.Items, 1)
}

func TestGetMyStandingReturnsOnlyAuthenticatedParticipant(t *testing.T) {
	service, standings, contests := newStandingTestEnv(t)
	ctx := context.Background()
	contestID, userID := testutil.ID(1), testutil.ID(20)
	contests.EXPECT().GetPublic(ctx, contestID).Return(&entity.Contest{
		ID: contestID, State: entity.ContestRunning, StandingsProjectionVersion: 6,
	}, nil)
	standings.EXPECT().FindByContest(ctx, contestID).Return([]*entity.ContestStanding{
		{ContestID: contestID, UserID: testutil.ID(10), Username: "alice", SolvedCount: 2},
		{ContestID: contestID, UserID: userID, Username: "bob", SolvedCount: 1},
	}, nil)

	response, err := service.GetMyStanding(ctx, contestID, userID)

	require.NoError(t, err)
	require.Equal(t, int64(6), response.ProjectionVersion)
	require.NotNil(t, response.Item)
	require.Equal(t, userID, response.Item.UserID)
	require.Equal(t, "bob", response.Item.Username)
}

func TestGetFinalStandingsPreservesRepositoryFailure(t *testing.T) {
	service, standings, _ := newStandingTestEnv(t)
	databaseError := errors.New("database unavailable")
	standings.EXPECT().FindByContest(gomock.Any(), testutil.ID(1)).Return(nil, databaseError)

	response, err := service.GetFinalStandings(context.Background(), testutil.ID(1))

	require.Nil(t, response)
	require.ErrorIs(t, err, databaseError)
}

func TestProjectStandingAtFreezeKeepsOnlyAcceptedBeforeCutoff(t *testing.T) {
	contestID, userID := testutil.ID(1), testutil.ID(10)
	beforeProblem := testutil.ID(100)
	afterProblem := testutil.ID(200)
	unsolvedProblem := testutil.ID(300)
	standing := &entity.ContestStanding{
		ContestID: contestID,
		UserID:    userID,
		ProblemResults: map[string]any{
			beforeProblem.String(): map[string]any{
				"ac": true, "wa_count": 2, "time": 300, "time_ms": int64(300_000),
			},
			afterProblem.String(): map[string]any{
				"ac": true, "wa_count": 0, "time": 700, "time_ms": int64(700_000),
			},
			unsolvedProblem.String(): map[string]any{
				"ac": false, "wa_count": 3, "time": 0, "time_ms": int64(0),
			},
		},
	}

	projected, err := projectStandingAtFreeze(standing, 600_000)

	require.NoError(t, err)
	require.Equal(t, 1, projected.SolvedCount)
	require.Contains(t, projected.ProblemResults, beforeProblem.String())
	require.NotContains(t, projected.ProblemResults, afterProblem.String())
	require.NotContains(t, projected.ProblemResults, unsolvedProblem.String())
}

func TestToRankedStandingResponsesRejectsInvalidProjection(t *testing.T) {
	response, err := toRankedStandingResponses([]*entity.ContestStanding{{
		ContestID: testutil.ID(1), UserID: uuid.Nil,
	}})

	require.Nil(t, response)
	require.ErrorIs(t, err, errInvalidStandingProjection)
}
