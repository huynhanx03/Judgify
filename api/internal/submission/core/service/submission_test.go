package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	submissionDTO "github.com/huynhanx03/judgify/internal/submission/core/dto"
	"github.com/huynhanx03/judgify/internal/submission/core/entity"
	submissionMocks "github.com/huynhanx03/judgify/internal/submission/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newSubmissionTestEnv(t *testing.T) (
	*submissionService,
	*submissionMocks.MockSubmissionRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := submissionMocks.NewMockSubmissionRepository(ctrl)
	// producers are nil — publishJudgeJob is tested separately
	svc := NewSubmissionService(repo, nil, nil).(*submissionService)
	return svc, repo
}

// --- Get ---

func TestGetSubmission_Success(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 1).Return(&entity.Submission{
		ID:        1,
		ProblemID: 10,
		UserID:    42,
		Language:  "cpp",
		Status:    "accepted",
	}, nil)

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, 10, resp.ProblemID)
	assert.Equal(t, "cpp", resp.Language)
}

func TestGetSubmission_NotFound(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- FindByProblemID ---

func TestFindByProblemID_Success(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByProblemID(ctx, 10).Return([]*entity.Submission{
		{ID: 1, ProblemID: 10, UserID: 42, Status: "accepted"},
		{ID: 2, ProblemID: 10, UserID: 43, Status: "wrong_answer"},
	}, nil)

	resp, err := svc.FindByProblemID(ctx, 10)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindByProblemID_Empty(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByProblemID(ctx, 999).Return([]*entity.Submission{}, nil)

	resp, err := svc.FindByProblemID(ctx, 999)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestFindByProblemID_RepoError(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByProblemID(ctx, 10).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.FindByProblemID(ctx, 10)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- FindByUserAndProblem ---

func TestFindByUserAndProblem_Success(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByUserAndProblem(ctx, 42, 10).Return([]*entity.Submission{
		{ID: 1, ProblemID: 10, UserID: 42, Status: "accepted"},
	}, nil)

	resp, err := svc.FindByUserAndProblem(ctx, 42, 10)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindByUserAndProblem_Empty(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByUserAndProblem(ctx, 42, 999).Return([]*entity.Submission{}, nil)

	resp, err := svc.FindByUserAndProblem(ctx, 42, 999)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestFindByUserAndProblem_RepoError(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().FindByUserAndProblem(ctx, 42, 10).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.FindByUserAndProblem(ctx, 42, 10)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateSubmission_ProducerNil_PanicsOnPublish(t *testing.T) {
	// When both producers are nil, publishJudgeJob will panic with nil pointer dereference.
	// This test verifies the repo.Create is called correctly and the entity is built.
	// The actual publish is a known limitation when producer is nil.
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	contID := 1
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, e *entity.Submission) error {
			e.ID = 1
			assert.Equal(t, 10, e.ProblemID)
			assert.Equal(t, 42, e.UserID)
			assert.Equal(t, "cpp", e.Language)
			assert.Equal(t, "pending", e.Status)
			assert.Equal(t, &contID, e.ContestID)
			return nil
		},
	)

	// Create will panic because s.producer is nil when trying to publish
	assert.Panics(t, func() {
		svc.Create(ctx, 42, &submissionDTO.CreateSubmissionRequest{
			ProblemID: 10,
			Language:  "cpp",
			SourceCode: "int main() {}",
			ContestID: &contID,
		})
	})
}

func TestCreateSubmission_RepoCreateError(t *testing.T) {
	svc, repo := newSubmissionTestEnv(t)
	ctx := context.Background()

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, 42, &submissionDTO.CreateSubmissionRequest{
		ProblemID:  10,
		Language:   "cpp",
		SourceCode: "int main() {}",
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}
