package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"

	"github.com/huynhanx03/judgify/internal/ports"
	submissionDTO "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	submissionMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newSubmissionTestEnv(t *testing.T) (
	*submissionService,
	*submissionMocks.MockSubmissionRepository,
) {
	t.Helper()
	repository := submissionMocks.NewMockSubmissionRepository(gomock.NewController(t))
	return NewSubmissionService(repository).(*submissionService), repository
}

func TestGetSubmissionReturnsOnlyOwnedDetail(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	repository.EXPECT().GetOwned(ctx, testutil.ID(1), testutil.ID(42)).Return(&entity.Submission{
		ID: testutil.ID(1), ProblemID: testutil.ID(10), UserID: testutil.ID(42),
		Language: "cpp17", SourceCode: "private source", Status: "accepted",
	}, nil)

	response, err := service.Get(ctx, testutil.ID(42), testutil.ID(1))

	assert.NoError(t, err)
	if assert.NotNil(t, response) {
		assert.Equal(t, testutil.ID(1), response.ID)
		assert.Equal(t, testutil.ID(10), response.ProblemID)
		assert.Equal(t, "cpp17", response.Language)
		assert.Equal(t, "private source", response.SourceCode)
	}
}

func TestGetSubmissionPropagatesRepositoryFailure(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	wantErr := apperr.New(apperr.CodeNotFound, "not found", nil)
	repository.EXPECT().GetOwned(ctx, testutil.ID(999), testutil.ID(42)).Return(nil, wantErr)

	response, err := service.Get(ctx, testutil.ID(42), testutil.ID(999))

	assert.Nil(t, response)
	assert.ErrorIs(t, err, wantErr)
}

func TestFindByProblemIDDoesNotExposeSource(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	repository.EXPECT().FindByProblemID(ctx, testutil.ID(10)).Return([]*entity.Submission{
		{ID: testutil.ID(1), ProblemID: testutil.ID(10), UserID: testutil.ID(42), SourceCode: "must not leak", Status: "accepted"},
		{ID: testutil.ID(2), ProblemID: testutil.ID(10), UserID: testutil.ID(43), Status: "wrong_answer"},
	}, nil)

	response, err := service.FindByProblemID(ctx, testutil.ID(10))

	assert.NoError(t, err)
	assert.Len(t, response, 2)
	encoded, marshalErr := json.Marshal(response[0])
	assert.NoError(t, marshalErr)
	assert.NotContains(t, string(encoded), "source_code")
	assert.NotContains(t, string(encoded), "must not leak")
}

func TestFindMyByProblemReturnsOpaqueCursorPage(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	contestID := testutil.ID(30)
	after := ports.SubmissionCursor{CreatedAt: createdAt.Add(time.Minute), ID: testutil.ID(9)}
	cursor, err := encodeSubmissionCursor(after)
	assert.NoError(t, err)
	last := &entity.Submission{
		ID: testutil.ID(2), ProblemID: testutil.ID(10), UserID: testutil.ID(42),
		Language: "go", ContestID: &contestID, Status: "accepted", CreatedAt: createdAt,
	}
	repository.EXPECT().FindMyByProblem(ctx, ports.MySubmissionQuery{
		UserID: testutil.ID(42), ProblemID: testutil.ID(10),
		ContestID: &contestID, After: &after, Limit: 2,
	}).Return(ports.MySubmissionPage{
		Submissions: []*entity.Submission{
			{
				ID: testutil.ID(1), ProblemID: testutil.ID(10), UserID: testutil.ID(42),
				Language: "go", ContestID: &contestID, Status: "judging",
				CreatedAt: createdAt.Add(time.Second),
			},
			last,
		},
		Next: &ports.SubmissionCursor{CreatedAt: last.CreatedAt, ID: last.ID},
	}, nil)

	response, err := service.FindMyByProblem(ctx, testutil.ID(42), &submissionDTO.MySubmissionsRequest{
		ProblemID: testutil.ID(10), ContestID: &contestID, Cursor: cursor, Limit: 2,
	})

	assert.NoError(t, err)
	if assert.NotNil(t, response) {
		assert.Len(t, response.Items, 2)
		assert.True(t, response.HasMore)
		if assert.NotNil(t, response.NextCursor) {
			assert.NotEqual(t, cursor, *response.NextCursor)
		}
	}
}

func TestFindMyByProblemDefaultsBoundAndReturnsExplicitEmptyPage(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	repository.EXPECT().FindMyByProblem(ctx, ports.MySubmissionQuery{
		UserID: testutil.ID(42), ProblemID: testutil.ID(999),
		Limit: ports.DefaultMySubmissionPageLimit,
	}).Return(ports.MySubmissionPage{Submissions: []*entity.Submission{}}, nil)

	response, err := service.FindMyByProblem(ctx, testutil.ID(42), &submissionDTO.MySubmissionsRequest{
		ProblemID: testutil.ID(999),
	})

	assert.NoError(t, err)
	if assert.NotNil(t, response) {
		assert.Empty(t, response.Items)
		assert.False(t, response.HasMore)
		assert.Nil(t, response.NextCursor)
	}
}

func TestFindMyByProblemRejectsInvalidCursorBeforeRepository(t *testing.T) {
	service, _ := newSubmissionTestEnv(t)
	response, err := service.FindMyByProblem(
		context.Background(),
		testutil.ID(42),
		&submissionDTO.MySubmissionsRequest{ProblemID: testutil.ID(10), Cursor: "forged"},
	)

	assert.Nil(t, response)
	var appError *apperr.AppError
	if assert.ErrorAs(t, err, &appError) {
		assert.Equal(t, apperr.CodeBadRequest, appError.Code)
	}
}

func TestFindMyByProblemRejectsOutOfOrderRepositoryProjection(t *testing.T) {
	service, repository := newSubmissionTestEnv(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	query := ports.MySubmissionQuery{
		UserID: testutil.ID(42), ProblemID: testutil.ID(10), Limit: 2,
	}
	repository.EXPECT().FindMyByProblem(ctx, query).Return(
		ports.MySubmissionPage{Submissions: []*entity.Submission{
			{
				ID: testutil.ID(1), ProblemID: query.ProblemID, UserID: query.UserID,
				Language: "go", Status: "accepted", CreatedAt: createdAt,
			},
			{
				ID: testutil.ID(2), ProblemID: query.ProblemID, UserID: query.UserID,
				Language: "go", Status: "accepted", CreatedAt: createdAt.Add(time.Second),
			},
		}},
		nil,
	)

	response, err := service.FindMyByProblem(
		ctx,
		query.UserID,
		&submissionDTO.MySubmissionsRequest{ProblemID: query.ProblemID, Limit: query.Limit},
	)

	assert.Nil(t, response)
	var appError *apperr.AppError
	if assert.ErrorAs(t, err, &appError) {
		assert.Equal(t, apperr.CodeInternalServer, appError.Code)
	}
}
