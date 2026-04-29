package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	"github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	problemMocks "github.com/huynhanx03/judgify/internal/problem/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
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

	svc := NewProblemService(problemRepo, tagRepo, diffRepo).(*problemService)
	return svc, problemRepo, tagRepo, diffRepo
}

// --- Get ---

func TestGetProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Get(ctx, 1).Return(&entity.Problem{
		ID:           1,
		Title:        "Two Sum",
		Description:  "Find two numbers that add up to target",
		DifficultyID: 2,
	}, nil)

	resp, err := svc.Get(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
	assert.Equal(t, "Two Sum", resp.Title)
}

func TestGetProblem_NotFound(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "problem not found", nil))

	resp, err := svc.Get(ctx, 999)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Exists(ctx, 1).Return(true, nil)
	problemRepo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteProblem_NotFound(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Exists(ctx, 999).Return(false, nil)

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjProblem)
}

func TestDeleteProblem_ExistsCheckError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Exists(ctx, 1).Return(false, apperr.New(response.CodeDatabaseError, "db error", nil))

	err := svc.Delete(ctx, 1)
	assert.Error(t, err)
}

// --- Create ---

func TestCreateProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			p.ID = 1
			return nil
		},
	)

	problemRepo.EXPECT().AddTags(gomock.Any(), 1, []int{10, 20}).Return(nil)

	problemRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Problem{
		ID:           1,
		Title:        "Two Sum",
		DifficultyID: 2,
	}, nil)

	resp, err := svc.Create(ctx, 1, &dto.CreateProblemRequest{
		Title:        "Two Sum",
		Description:  "Find two numbers that add up to target",
		DifficultyID: 2,
		TagIDs:       []int{10, 20},
	})

	assert.NoError(t, err)
	assert.Equal(t, "Two Sum", resp.Title)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateProblem_WithoutTags(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			p.ID = 2
			return nil
		},
	)

	// AddTags should NOT be called when TagIDs is empty

	problemRepo.EXPECT().Get(gomock.Any(), 2).Return(&entity.Problem{
		ID:           2,
		Title:        "Simple Problem",
		DifficultyID: 1,
	}, nil)

	resp, err := svc.Create(ctx, 1, &dto.CreateProblemRequest{
		Title:        "Simple Problem",
		Description:  "Easy one",
		DifficultyID: 1,
		TagIDs:       nil,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Simple Problem", resp.Title)
}

func TestCreateProblem_RepoCreateError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Create(ctx, 1, &dto.CreateProblemRequest{
		Title:        "Fail",
		Description:  "desc",
		DifficultyID: 1,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateProblem_AddTagsError(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *entity.Problem) error {
			p.ID = 1
			return nil
		},
	)

	problemRepo.EXPECT().AddTags(gomock.Any(), 1, []int{99}).Return(apperr.New(response.CodeDatabaseError, "tag error", nil))

	resp, err := svc.Create(ctx, 1, &dto.CreateProblemRequest{
		Title:        "Fail Tags",
		Description:  "desc",
		DifficultyID: 1,
		TagIDs:       []int{99},
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Update ---

func TestUpdateProblem_Success(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Get(ctx, 1).Return(&entity.Problem{
		ID:           1,
		Title:        "Two Sum",
		DifficultyID: 2,
	}, nil)

	problemRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	problemRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Problem{
		ID:           1,
		Title:        "Three Sum",
		DifficultyID: 3,
	}, nil)

	newTitle := "Three Sum"
	newDiff := 3
	resp, err := svc.Update(ctx, 1, &dto.UpdateProblemRequest{
		Title:        &newTitle,
		DifficultyID: &newDiff,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Three Sum", resp.Title)
}

func TestUpdateProblem_NotFound(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))

	newTitle := "Nope"
	resp, err := svc.Update(ctx, 999, &dto.UpdateProblemRequest{
		Title: &newTitle,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateProblem_WithTagsReplace(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	problemRepo.EXPECT().Get(ctx, 1).Return(&entity.Problem{
		ID:           1,
		Title:        "Two Sum",
		DifficultyID: 2,
	}, nil)

	problemRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newTags := []int{30, 40}
	problemRepo.EXPECT().ReplaceTags(gomock.Any(), 1, newTags).Return(nil)

	problemRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Problem{
		ID:    1,
		Title: "Two Sum",
	}, nil)

	resp, err := svc.Update(ctx, 1, &dto.UpdateProblemRequest{
		TagIDs: &newTags,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Find ---

func TestFindProblems_EmptyResult(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)
	ctx := context.Background()

	records := []*entity.Problem{}
	problemRepo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
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

	problemRepo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
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

	problemRepo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "db error", nil))

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindProblems_WithAuthContext(t *testing.T) {
	svc, problemRepo, _, _ := newProblemTestEnv(t)

	// Inject userID into context
	ctx := context.WithValue(context.Background(), struct{}{}, nil) // no userID — no enrichment

	records := []*entity.Problem{
		{ID: 1, Title: "Two Sum", DifficultyID: 1},
		{ID: 2, Title: "Three Sum", DifficultyID: 2},
	}

	problemRepo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Problem]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}
