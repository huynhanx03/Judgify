package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/constant"
	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	contestmocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newContestTestEnv(t *testing.T) (
	*contestService,
	*contestmocks.MockContestRepository,
	*contestmocks.MockRegistrationRepository,
) {
	t.Helper()
	controller := gomock.NewController(t)
	contestRepository := contestmocks.NewMockContestRepository(controller)
	registrationRepository := contestmocks.NewMockRegistrationRepository(controller)
	service := NewContestService(
		contestRepository,
		registrationRepository,
		nil,
	).(*contestService)
	return service, contestRepository, registrationRepository
}

func validContestDraftRequest() contract.ContestDraftRequest {
	start := time.Now().UTC().Add(24 * time.Hour)
	return contract.ContestDraftRequest{
		Reason:           "reviewed contest draft",
		Title:            "Weekly Contest",
		Description:      "Practice immutable contest authoring.",
		StartTime:        start,
		EndTime:          start.Add(2 * time.Hour),
		RegistrationMode: string(entity.RegistrationModeOpen),
		Problems: []contract.ContestDraftProblemRequest{
			{ProblemID: testutil.ID(10), Alias: "A", VisibleBeforeStart: true},
			{ProblemID: testutil.ID(20), Alias: "B"},
		},
	}
}

func TestCreateContestPersistsOneValidatedAggregate(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	authorID := testutil.ID(1)
	request := &contract.CreateContestRequest{ContestDraftRequest: validContestDraftRequest()}

	repository.EXPECT().CreateDraft(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, contest *entity.Contest, problems []entity.ContestDraftProblem) error {
			require.Equal(t, authorID, contest.AuthorID)
			require.Equal(t, entity.ContestDraft, contest.EffectiveState())
			require.Equal(t, int64(1), contest.Version)
			require.Len(t, problems, 2)
			require.Equal(t, 1, problems[0].DisplayOrder)
			require.Equal(t, "A", problems[0].Alias)
			require.Equal(t, 2, problems[1].DisplayOrder)
			contest.ID = testutil.ID(100)
			problems[0].ProblemSlug = "two-sum"
			problems[0].ProblemTitle = "Two Sum"
			problems[1].ProblemSlug = "sort"
			problems[1].ProblemTitle = "Sort"
			return nil
		},
	)

	response, err := service.Create(ctx, authorID, request)

	require.NoError(t, err)
	require.Equal(t, testutil.ID(100), response.ID)
	require.Equal(t, string(entity.ContestDraft), response.Status)
	require.Equal(t, 2, response.ProblemCount)
	require.Len(t, response.DraftProblems, 2)
	require.Equal(t, "two-sum", response.DraftProblems[0].ProblemSlug)
}

func TestCreateContestRejectsInvalidWindowBeforePersistence(t *testing.T) {
	service, _, _ := newContestTestEnv(t)
	request := validContestDraftRequest()
	request.EndTime = request.StartTime

	response, err := service.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateContestRequest{ContestDraftRequest: request},
	)

	require.Nil(t, response)
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeValidationFailed, applicationError.Code)
}

func TestGetPublicContestReturnsFrozenProblemsAndCallerRegistration(t *testing.T) {
	service, repository, registrations := newContestTestEnv(t)
	ctx := context.Background()
	contestID, userID := testutil.ID(1), testutil.ID(42)
	contest := &entity.Contest{
		ID:      contestID,
		Title:   "Weekly Contest",
		State:   entity.ContestUpcoming,
		Version: 3,
	}
	registrationID := testutil.ID(43)
	repository.EXPECT().GetPublic(ctx, contestID).Return(contest, nil)
	repository.EXPECT().GetReadModels(ctx, []uuid.UUID{contestID}, false).Return(
		map[uuid.UUID]ports.ContestReadModel{
			contestID: {ParticipantCount: 5, ProblemCount: 2},
		},
		nil,
	)
	repository.EXPECT().GetContestProblemProjections(ctx, contestID).Return(
		[]ports.ContestProblemProjection{
			{
				ContestProblemID:   testutil.ID(11),
				ProblemID:          testutil.ID(10),
				ProblemSlug:        "two-sum",
				ProblemTitle:       "Two Sum",
				DisplayOrder:       1,
				Alias:              "A",
				VisibleBeforeStart: true,
			},
			{
				ContestProblemID: testutil.ID(21),
				ProblemID:        testutil.ID(20),
				DisplayOrder:     2,
				Alias:            "B",
			},
		},
		nil,
	)
	registrations.EXPECT().GetProjection(ctx, contestID, userID).Return(
		&ports.RegistrationProjection{
			ID:      registrationID,
			Status:  entity.RegistrationRegistered,
			Version: 2,
		},
		nil,
	)

	response, err := service.GetPublic(ctx, contestID, userID)

	require.NoError(t, err)
	require.Equal(t, 5, response.ParticipantCount)
	require.Equal(t, 2, response.ProblemCount)
	require.Len(t, response.Problems, 1, "hidden upcoming memberships must not leak")
	require.Equal(t, testutil.ID(11), response.Problems[0].ContestProblemID)
	require.NotNil(t, response.IsRegistered)
	require.True(t, *response.IsRegistered)
	require.NotNil(t, response.Registration)
	require.Equal(t, registrationID, response.Registration.ID)
}

func TestGetPublicContestPreservesNotFound(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	notFound := apperr.New(apperr.CodeNotFound, "contest not found", nil)
	repository.EXPECT().GetPublic(gomock.Any(), testutil.ID(99)).Return(nil, notFound)

	response, err := service.GetPublic(context.Background(), testutil.ID(99), uuid.Nil)

	require.Nil(t, response)
	require.ErrorIs(t, err, notFound)
}

func TestFindPublicContestsUsesRestrictedBatchedReadModel(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	query := &d.QueryOptions{Filters: []d.SearchFilter{{
		Key: "status", Value: constant.ContestStatusDraft, Type: "exact",
	}}}
	contest := &entity.Contest{
		ID: testutil.ID(1), Title: "Upcoming", State: entity.ContestUpcoming,
	}
	records := []*entity.Contest{contest}
	repository.EXPECT().FindPublic(ctx, query).Return(
		&d.Paginated[*entity.Contest]{Records: &records},
		nil,
	)
	repository.EXPECT().GetReadModels(ctx, []uuid.UUID{contest.ID}, false).Return(
		map[uuid.UUID]ports.ContestReadModel{
			contest.ID: {ParticipantCount: 3, ProblemCount: 2},
		},
		nil,
	)

	page, err := service.FindPublic(ctx, query)

	require.NoError(t, err)
	require.Len(t, *page.Records, 1)
	require.Equal(t, 3, (*page.Records)[0].ParticipantCount)
	require.Equal(t, 2, (*page.Records)[0].ProblemCount)
	require.Nil(t, (*page.Records)[0].DraftProblems)
}

func TestAdminDraftReadsIncludeOrderedDraftProjection(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	draft := &entity.Contest{
		ID: contestID, Title: "Private draft", State: entity.ContestDraft,
	}
	draftProblems := []entity.ContestDraftProblem{{
		ProblemID: testutil.ID(10), ProblemSlug: "two-sum",
		ProblemTitle: "Two Sum", DisplayOrder: 1, Alias: "A",
	}}
	repository.EXPECT().Get(ctx, contestID).Return(draft, nil)
	repository.EXPECT().GetReadModels(ctx, []uuid.UUID{contestID}, true).Return(
		map[uuid.UUID]ports.ContestReadModel{
			contestID: {ParticipantCount: 0, DraftProblems: draftProblems},
		},
		nil,
	)

	response, err := service.GetAdmin(ctx, contestID)

	require.NoError(t, err)
	require.Equal(t, string(entity.ContestDraft), response.Status)
	require.Equal(t, 1, response.ProblemCount)
	require.Len(t, response.DraftProblems, 1)
	require.Equal(t, "A", response.DraftProblems[0].Alias)
}

func TestUpdateContestUsesOptimisticDraftFence(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	request := validContestDraftRequest()
	request.Title = "Updated Contest"
	current := &entity.Contest{
		ID:                     contestID,
		AuthorID:               testutil.ID(2),
		State:                  entity.ContestDraft,
		Version:                7,
		ScoringPolicy:          entity.ScoringPolicyICPC,
		ScoringPolicyVersion:   1,
		NextSubmissionSequence: 1,
	}
	repository.EXPECT().Get(ctx, contestID).Return(current, nil)
	repository.EXPECT().UpdateDraft(ctx, gomock.Any(), int64(7), gomock.Any()).DoAndReturn(
		func(_ context.Context, contest *entity.Contest, expectedVersion int64, problems []entity.ContestDraftProblem) error {
			require.Equal(t, int64(7), expectedVersion)
			require.Equal(t, "Updated Contest", contest.Title)
			require.Len(t, problems, 2)
			contest.Version = expectedVersion + 1
			return nil
		},
	)
	repository.EXPECT().GetReadModels(ctx, []uuid.UUID{contestID}, true).Return(
		map[uuid.UUID]ports.ContestReadModel{
			contestID: {DraftProblems: []entity.ContestDraftProblem{
				{ProblemID: testutil.ID(10), DisplayOrder: 1, Alias: "A"},
				{ProblemID: testutil.ID(20), DisplayOrder: 2, Alias: "B"},
			}},
		},
		nil,
	)

	response, err := service.Update(ctx, contestID, &contract.UpdateContestRequest{
		ID:                  contestID,
		ExpectedVersion:     7,
		ContestDraftRequest: request,
	})

	require.NoError(t, err)
	require.Equal(t, int64(8), response.Version)
	require.Equal(t, "Updated Contest", response.Title)
}

func TestUpdateContestMapsVersionRaceToConflict(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	request := validContestDraftRequest()
	repository.EXPECT().Get(ctx, contestID).Return(&entity.Contest{
		ID:                     contestID,
		AuthorID:               testutil.ID(2),
		State:                  entity.ContestDraft,
		Version:                4,
		ScoringPolicy:          entity.ScoringPolicyICPC,
		ScoringPolicyVersion:   1,
		NextSubmissionSequence: 1,
	}, nil)

	response, err := service.Update(ctx, contestID, &contract.UpdateContestRequest{
		ExpectedVersion:     3,
		ContestDraftRequest: request,
	})

	require.Nil(t, response)
	require.ErrorIs(t, err, entity.ErrContestVersionConflict)
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeConflict, applicationError.Code)
}

func TestDeleteContestDeletesOnlyDraftWithAuditedReason(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	repository.EXPECT().Get(ctx, contestID).Return(&entity.Contest{
		ID: contestID, State: entity.ContestDraft, Version: 2,
	}, nil)
	repository.EXPECT().DeleteDraft(ctx, contestID).Return(nil)

	require.NoError(t, service.Delete(ctx, contestID, "obsolete draft"))
}

func TestDeleteContestRejectsPublishedAggregate(t *testing.T) {
	service, repository, _ := newContestTestEnv(t)
	ctx := context.Background()
	contestID := testutil.ID(1)
	repository.EXPECT().Get(ctx, contestID).Return(&entity.Contest{
		ID: contestID, State: entity.ContestUpcoming, Version: 2,
	}, nil)

	err := service.Delete(ctx, contestID, "must use cancellation")

	require.ErrorIs(t, err, entity.ErrContestNotDraft)
	var applicationError *apperr.AppError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, apperr.CodeConflict, applicationError.Code)
}
