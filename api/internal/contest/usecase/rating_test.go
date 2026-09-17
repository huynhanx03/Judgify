package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/platform/eventing"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	contestmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

type ratingOutboxFake struct {
	events []eventing.Event
	err    error
}

func (fake *ratingOutboxFake) Append(_ context.Context, events ...eventing.Event) error {
	if fake.err != nil {
		return fake.err
	}
	fake.events = append(fake.events, events...)
	return nil
}

func newRatingTestEnv(t *testing.T) (
	*ratingService,
	*contestmocks.MockRatingLedgerRepository,
	*ratingOutboxFake,
) {
	t.Helper()
	ledger := contestmocks.NewMockRatingLedgerRepository(gomock.NewController(t))
	outbox := &ratingOutboxFake{}
	service := NewRatingService(ledger, testutil.TxManager{}, outbox).(*ratingService)
	return service, ledger, outbox
}

func TestGetContestRatingChangesProjectsCanonicalActiveRun(t *testing.T) {
	service, ledger, _ := newRatingTestEnv(t)
	ctx := context.Background()
	contestID := uuid.New()
	ledger.EXPECT().FindActiveByContest(ctx, contestID).Return(&entity.RatingRun{
		Results: []entity.RatingRunResult{
			{
				UserID:         uuid.New(),
				Username:       "alice",
				PreviousRating: 1000,
				NewRating:      1038,
				Rank:           1,
				Delta:          38,
			},
			{
				UserID:         uuid.New(),
				Username:       "bob",
				PreviousRating: 1500,
				NewRating:      1462,
				Rank:           2,
				Delta:          -38,
			},
		},
	}, nil)

	response, err := service.GetContestRatingChanges(ctx, contestID)

	require.NoError(t, err)
	require.Len(t, response, 2)
	require.Equal(t, "alice", response[0].Username)
	require.Equal(t, 38, response[0].Delta)
	require.Equal(t, "bob", response[1].Username)
	require.Equal(t, -38, response[1].Delta)
}

func TestGetContestRatingChangesReturnsEmptyBeforeSettlement(t *testing.T) {
	service, ledger, _ := newRatingTestEnv(t)
	contestID := uuid.New()
	ledger.EXPECT().FindActiveByContest(gomock.Any(), contestID).Return(
		nil,
		ports.ErrRatingNotAvailable,
	)

	response, err := service.GetContestRatingChanges(context.Background(), contestID)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Empty(t, response)
}

func TestGetContestRatingChangesPreservesRepositoryFailure(t *testing.T) {
	service, ledger, _ := newRatingTestEnv(t)
	contestID := uuid.New()
	databaseError := errors.New("database unavailable")
	ledger.EXPECT().FindActiveByContest(gomock.Any(), contestID).Return(nil, databaseError)

	response, err := service.GetContestRatingChanges(context.Background(), contestID)

	require.Nil(t, response)
	require.ErrorIs(t, err, databaseError)
}

func TestCalculateRatingReplaysCanonicalRunWithoutDuplicateEvent(t *testing.T) {
	service, ledger, outbox := newRatingTestEnv(t)
	contestID := uuid.New()
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	service.clock = func() time.Time { return now }
	ledger.EXPECT().Prepare(gomock.Any(), contestID, now).Return(
		&entity.RatingSettlementSnapshot{
			ContestID:     contestID,
			IdempotentRun: &entity.RatingRun{ID: uuid.New(), RatingSequence: 4},
		},
		nil,
	)

	require.NoError(t, service.CalculateRating(context.Background(), contestID))
	require.Empty(t, outbox.events)
}

func TestCalculateRatingAppliesImmutableRunAndAppendsOneEvent(t *testing.T) {
	service, ledger, outbox := newRatingTestEnv(t)
	ctx := context.Background()
	contestID := uuid.New()
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	service.clock = func() time.Time { return now }
	firstUser, secondUser := uuid.New(), uuid.New()
	snapshot := &entity.RatingSettlementSnapshot{
		ContestID:                  contestID,
		RatingSequence:             4,
		RatingSequenceSlotID:       uuid.New(),
		RunVersion:                 1,
		StandingsProjectionVersion: 2,
		AlgorithmVersion:           RatingAlgorithmVersion,
		ProjectionRevision:         3,
		ProjectionGenerationID:     uuid.New(),
		RunChainID:                 uuid.New(),
		ProjectionChecksum:         make([]byte, 32),
		RunChainChecksum:           make([]byte, 32),
		Participants: []entity.RatingParticipantSnapshot{
			{
				UserID:                firstUser,
				Username:              "alice",
				SolvedCount:           5,
				PenaltySeconds:        100,
				PreviousRating:        1000,
				PreviousRatedContests: 0,
			},
			{
				UserID:                secondUser,
				Username:              "bob",
				SolvedCount:           3,
				PenaltySeconds:        200,
				PreviousRating:        1500,
				PreviousRatedContests: 0,
			},
		},
	}
	ledger.EXPECT().Prepare(ctx, contestID, now).Return(snapshot, nil)
	runID := uuid.New()
	ledger.EXPECT().Apply(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, command entity.ApplyRatingRun) (*entity.RatingRun, error) {
			require.Equal(t, snapshot, &command.Snapshot)
			require.Equal(t, now, command.AppliedAt)
			require.NotEqual(t, uuid.Nil, command.RunID)
			require.Len(t, command.InputChecksum, 32)
			require.Len(t, command.Results, 2)
			require.Equal(t, firstUser, command.Results[0].UserID)
			require.Equal(t, 1, command.Results[0].Rank)
			require.Equal(t, 1038, command.Results[0].NewRating)
			require.Equal(t, secondUser, command.Results[1].UserID)
			require.Equal(t, 2, command.Results[1].Rank)
			require.Equal(t, 1462, command.Results[1].NewRating)
			return &entity.RatingRun{
				ID:                         runID,
				ContestID:                  contestID,
				RatingSequence:             4,
				RunVersion:                 1,
				StandingsProjectionVersion: 2,
				AlgorithmVersion:           RatingAlgorithmVersion,
				ParticipantCount:           2,
			}, nil
		},
	)

	require.NoError(t, service.CalculateRating(ctx, contestID))
	require.Len(t, outbox.events, 1)
	require.Equal(t, contract.EventRatingCalculatedV1, outbox.events[0].Type)
	require.Equal(t, contract.AggregateRatingRun, outbox.events[0].AggregateType)
	require.Equal(t, runID, outbox.events[0].AggregateID)
	require.Equal(t, int64(1), outbox.events[0].AggregateVersion)
	require.Equal(t, now, outbox.events[0].OccurredAt)
}

func TestCalculateRatingReturnsPrepareFailureWithoutEvent(t *testing.T) {
	service, ledger, outbox := newRatingTestEnv(t)
	contestID := uuid.New()
	databaseError := errors.New("database unavailable")
	ledger.EXPECT().Prepare(gomock.Any(), contestID, gomock.Any()).Return(nil, databaseError)

	err := service.CalculateRating(context.Background(), contestID)

	require.ErrorIs(t, err, databaseError)
	require.Empty(t, outbox.events)
}

func TestCalculateRatingReturnsOutboxFailureToRollbackTransaction(t *testing.T) {
	service, ledger, outbox := newRatingTestEnv(t)
	contestID := uuid.New()
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	service.clock = func() time.Time { return now }
	outbox.err = errors.New("outbox unavailable")
	snapshot := &entity.RatingSettlementSnapshot{
		ContestID:                  contestID,
		RatingSequence:             1,
		RunVersion:                 1,
		StandingsProjectionVersion: 1,
		AlgorithmVersion:           RatingAlgorithmVersion,
	}
	ledger.EXPECT().Prepare(gomock.Any(), contestID, now).Return(snapshot, nil)
	ledger.EXPECT().Apply(gomock.Any(), gomock.Any()).Return(&entity.RatingRun{
		ID:                         uuid.New(),
		ContestID:                  contestID,
		RatingSequence:             1,
		RunVersion:                 1,
		StandingsProjectionVersion: 1,
		AlgorithmVersion:           RatingAlgorithmVersion,
	}, nil)

	err := service.CalculateRating(context.Background(), contestID)

	require.ErrorIs(t, err, outbox.err)
}

func TestCalculateRatingDeltasIsDeterministicAndTieAware(t *testing.T) {
	firstUser := uuid.MustParse("00000000-0000-7000-8000-000000000001")
	secondUser := uuid.MustParse("00000000-0000-7000-8000-000000000002")
	thirdUser := uuid.MustParse("00000000-0000-7000-8000-000000000003")
	inputs := []RatingInput{
		{UserID: thirdUser, SolvedCount: 1, PenaltySeconds: 300, PreviousRating: 1200},
		{UserID: secondUser, SolvedCount: 2, PenaltySeconds: 100, PreviousRating: 1200},
		{UserID: firstUser, SolvedCount: 2, PenaltySeconds: 100, PreviousRating: 1200},
	}

	output, err := CalculateRatingDeltas(inputs)

	require.NoError(t, err)
	require.Len(t, output, 3)
	require.Equal(t, firstUser, output[0].UserID, "UUID breaks an exact result tie deterministically")
	require.Equal(t, secondUser, output[1].UserID)
	require.Equal(t, 1, output[0].Rank)
	require.Equal(t, 1, output[1].Rank)
	require.Equal(t, 1.5, output[0].ActualRank)
	require.Equal(t, 1.5, output[1].ActualRank)
	require.Equal(t, 3, output[2].Rank)
	require.Equal(t, thirdUser, output[2].UserID)
}

func TestCalculateRatingDeltasRejectsDuplicateParticipant(t *testing.T) {
	userID := uuid.New()
	output, err := CalculateRatingDeltas([]RatingInput{
		{UserID: userID, PreviousRating: InitialRating},
		{UserID: userID, PreviousRating: InitialRating},
	})

	require.Nil(t, output)
	require.ErrorIs(t, err, ErrInvalidRatingInput)
}

func TestExpectedScoreByDiffCachesResult(t *testing.T) {
	cache := map[int]float64{}
	first := expectedScoreByDiff(120, cache)
	second := expectedScoreByDiff(120, cache)
	require.Equal(t, first, second)
	require.Len(t, cache, 1)
}

func TestCalculateExpectedRanksByBucketsHandlesUniformField(t *testing.T) {
	participants := make([]ratingParticipant, 5000)
	for index := range participants {
		participants[index].currentRating = 1500
	}

	expectedRanks := calculateExpectedRanksByBuckets(participants)
	expected := float64(len(participants)+1) / 2
	require.Len(t, expectedRanks, len(participants))
	for _, rank := range expectedRanks {
		require.InDelta(t, expected, rank, 1e-9)
	}
}

func TestLargeContestUsesBoundedApproximation(t *testing.T) {
	require.False(t, shouldUseApproximation(largeContestApproxThreshold))
	require.True(t, shouldUseApproximation(largeContestApproxThreshold+1))
}
