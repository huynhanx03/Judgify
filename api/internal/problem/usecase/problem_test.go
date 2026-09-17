package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	problemMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newProblemTestEnv(t *testing.T) (
	*problemService,
	*problemMocks.MockProblemRepository,
	*problemMocks.MockTagRepository,
	*problemMocks.MockDifficultyRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	problemRepo := problemMocks.NewMockProblemRepository(ctrl)
	tagRepo := problemMocks.NewMockTagRepository(ctrl)
	diffRepo := problemMocks.NewMockDifficultyRepository(ctrl)

	svc := NewProblemService(
		problemRepo,
		tagRepo,
		diffRepo,
		testutil.TxManager{},
	).(*problemService)
	return svc, problemRepo, tagRepo, diffRepo
}

type failingProblemTx struct {
	err error
}

func (transaction failingProblemTx) DoInTx(
	context.Context,
	func(context.Context) error,
) error {
	return transaction.err
}

type problemTxContextKey struct{}

type markedProblemTx struct{}

func (markedProblemTx) DoInTx(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	return fn(context.WithValue(ctx, problemTxContextKey{}, true))
}

type problemTxContextMatcher struct{}

func (problemTxContextMatcher) Matches(value any) bool {
	ctx, ok := value.(context.Context)
	return ok && ctx.Value(problemTxContextKey{}) == true
}

func (problemTxContextMatcher) String() string {
	return "context created by the problem transaction manager"
}

// --- Get ---

func TestGetProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().GetPublished(ctx, testutil.ID(1)).Return(&entity.Problem{
		ID:           testutil.ID(1),
		Title:        "Two Sum",
		Description:  "Find two numbers that add up to target",
		DifficultyID: testutil.ID(2),
	}, nil)

	resp, err := svc.Get(ctx, testutil.ID(1))

	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
	assert.Equal(t, "Two Sum", resp.Title)
}

func TestGetProblem_NotFound(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().GetPublished(ctx, testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "problem not found", nil))

	resp, err := svc.Get(ctx, testutil.ID(999))

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetAdminProblem_IncludesDraft(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()
	draft := &entity.Problem{
		ID:          testutil.ID(2),
		Title:       "Draft problem",
		IsPublished: false,
	}

	problemRepo.EXPECT().Get(ctx, draft.ID).Return(draft, nil)

	resp, err := svc.GetAdmin(ctx, draft.ID)

	assert.NoError(t, err)
	assert.Equal(t, draft.ID, resp.ID)
	assert.False(t, resp.IsPublished)
}

// --- Create ---

func TestCreateProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			if p.Slug != "two-sum" {
				t.Fatalf("problem slug = %q, want canonical explicit slug", p.Slug)
			}
			p.ID = testutil.ID(1)
			return nil
		},
	)

	problemRepo.EXPECT().AddTags(gomock.Any(), testutil.ID(1), []uuid.UUID{testutil.ID(10), testutil.ID(20)}).Return(nil)

	problemRepo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Problem{
		ID:           testutil.ID(1),
		Title:        "Two Sum",
		DifficultyID: testutil.ID(2),
	}, nil)

	resp, err := svc.Create(ctx, testutil.ID(1), &contract.CreateProblemCommand{
		Slug:         " Two__SUM ",
		Title:        "Two Sum",
		Description:  "Find two numbers that add up to target",
		DifficultyID: testutil.ID(2),
		TagIDs:       []uuid.UUID{testutil.ID(10), testutil.ID(20)},
	})

	assert.NoError(t, err)
	assert.Equal(t, "Two Sum", resp.Title)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateProblem_WithoutTags(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			p.ID = testutil.ID(2)
			return nil
		},
	)

	// AddTags should NOT be called when TagIDs is empty

	problemRepo.EXPECT().Get(gomock.Any(), testutil.ID(2)).Return(&entity.Problem{
		ID:           testutil.ID(2),
		Title:        "Simple Problem",
		DifficultyID: testutil.ID(1),
	}, nil)

	resp, err := svc.Create(ctx, testutil.ID(1), &contract.CreateProblemCommand{
		Slug:         "simple-problem",
		Title:        "Simple Problem",
		Description:  "Easy one",
		DifficultyID: testutil.ID(1),
		TagIDs:       nil,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Simple Problem", resp.Title)
}

func TestCreateProblem_RepoCreateError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, testutil.ID(1), &contract.CreateProblemCommand{
		Slug:         "fail",
		Title:        "Fail",
		Description:  "desc",
		DifficultyID: testutil.ID(1),
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateProblem_MapsSlugConflictToStableClientError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	problemRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(ports.ErrProblemSlugConflict)

	response, err := svc.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateProblemCommand{
			Slug:         "two-sum",
			Title:        "Two Sum",
			Description:  "Find a pair",
			DifficultyID: testutil.ID(2),
		},
	)

	assert.Nil(t, response)
	var applicationError *apperr.AppError
	assert.ErrorAs(t, err, &applicationError)
	assert.Equal(t, apperr.CodeConflict, applicationError.Code)
	assert.Equal(t, "problem slug is already in use", applicationError.Message)
	assert.Equal(
		t,
		map[string]any{"field": "slug", "reason": "already_exists"},
		applicationError.Details,
	)
	assert.ErrorIs(t, err, ports.ErrProblemSlugConflict)
}

func TestCreateProblem_DoesNotRelabelAnotherRepositoryConflictAsSlug(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	repositoryConflict := apperr.New(
		apperr.CodeConflict,
		"problem constraint failed",
		errors.New("another constraint"),
	)
	problemRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(repositoryConflict)

	response, err := svc.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateProblemCommand{
			Slug:         "unique-slug",
			Title:        "Unique Slug",
			Description:  "A different persistence conflict.",
			DifficultyID: testutil.ID(2),
		},
	)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, repositoryConflict)
	assert.NotErrorIs(t, err, ports.ErrProblemSlugConflict)
	var applicationError *apperr.AppError
	assert.ErrorAs(t, err, &applicationError)
	assert.Equal(t, "problem constraint failed", applicationError.Message)
	assert.Nil(t, applicationError.Details)
}

func TestCreateProblem_InvalidSlugDoesNotWrite(t *testing.T) {
	svc, _, _, _ := newProblemTestEnv(t)

	response, err := svc.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateProblemCommand{
			Slug:         "C++",
			Title:        "C++ Basics",
			Description:  "Learn the language",
			DifficultyID: testutil.ID(2),
		},
	)

	assert.Nil(t, response)
	assert.Error(t, err)
	var appError *apperr.AppError
	assert.ErrorAs(t, err, &appError)
	assert.Equal(t, apperr.CodeBadRequest, appError.Code)
}

func TestCreateProblem_TransactionStartFailureDoesNotWrite(t *testing.T) {
	_, problemRepo, tagRepo, diffRepo := newProblemTestEnv(t)
	want := errors.New("transaction unavailable")
	svc := NewProblemService(
		problemRepo,
		tagRepo,
		diffRepo,
		failingProblemTx{err: want},
	).(*problemService)

	response, err := svc.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateProblemCommand{
			Slug:         "two-sum",
			Title:        "Two Sum",
			Description:  "Find a pair",
			DifficultyID: testutil.ID(2),
		},
	)

	assert.Nil(t, response)
	assert.ErrorIs(t, err, want)
}

func TestCreateProblem_UsesTransactionContextForEveryRepositoryCall(t *testing.T) {
	_, problemRepo, tagRepo, diffRepo := newProblemTestEnv(t)
	svc := NewProblemService(
		problemRepo,
		tagRepo,
		diffRepo,
		markedProblemTx{},
	).(*problemService)
	ctxMatcher := problemTxContextMatcher{}
	problemID := testutil.ID(11)
	tagIDs := []uuid.UUID{testutil.ID(21), testutil.ID(22)}

	gomock.InOrder(
		problemRepo.EXPECT().
			Create(ctxMatcher, gomock.Any()).
			DoAndReturn(func(_ context.Context, problem *entity.Problem) error {
				problem.ID = problemID
				return nil
			}),
		problemRepo.EXPECT().AddTags(ctxMatcher, problemID, tagIDs).Return(nil),
		problemRepo.EXPECT().Get(ctxMatcher, problemID).Return(&entity.Problem{
			ID:    problemID,
			Slug:  "transaction-context",
			Title: "Transaction Context",
		}, nil),
	)

	response, err := svc.Create(
		context.Background(),
		testutil.ID(1),
		&contract.CreateProblemCommand{
			Slug:         "transaction-context",
			Title:        "Transaction Context",
			Description:  "Every write and refetch shares one transaction.",
			DifficultyID: testutil.ID(2),
			TagIDs:       tagIDs,
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, problemID, response.ID)
}

func TestCreateProblem_AddTagsError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			p.ID = testutil.ID(1)
			return nil
		},
	)

	problemRepo.EXPECT().AddTags(gomock.Any(), testutil.ID(1), []uuid.UUID{testutil.ID(99)}).Return(apperr.New(apperr.CodeDatabaseError, "tag error", nil))

	resp, err := svc.Create(ctx, testutil.ID(1), &contract.CreateProblemCommand{
		Slug:         "fail-tags",
		Title:        "Fail Tags",
		Description:  "desc",
		DifficultyID: testutil.ID(1),
		TagIDs:       []uuid.UUID{testutil.ID(99)},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Find ---

func TestFindProblems_EmptyResult(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	records := []*entity.Problem{}
	problemRepo.EXPECT().FindPublished(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

func TestFindProblems_NilRecords(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().FindPublished(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
		Records:    nil,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestFindProblems_RepoError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().FindPublished(ctx, gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindProblems_WithAuthContext(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)

	type testContextKey struct{}
	// Inject userID into context
	ctx := context.WithValue(context.Background(), testContextKey{}, nil) // no userID — no enrichment

	records := []*entity.Problem{
		{ID: testutil.ID(1), Title: "Two Sum", DifficultyID: testutil.ID(1)},
		{ID: testutil.ID(2), Title: "Three Sum", DifficultyID: testutil.ID(2)},
	}

	problemRepo.EXPECT().FindPublished(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindAdminProblems_IncludesDrafts(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()
	records := []*entity.Problem{
		{ID: testutil.ID(1), Title: "Published", IsPublished: true},
		{ID: testutil.ID(2), Title: "Draft", IsPublished: false},
	}
	options := &d.QueryOptions{}

	problemRepo.EXPECT().Find(ctx, options).Return(&d.Paginated[*entity.Problem]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.FindAdmin(ctx, options)

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
	assert.False(t, (*resp.Records)[1].IsPublished)
}
