package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/contest/constant"
	"github.com/huynhanx03/judgify/internal/contest/core/dto"
	"github.com/huynhanx03/judgify/internal/contest/core/entity"
	contestMocks "github.com/huynhanx03/judgify/internal/contest/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newContestTestEnv(t *testing.T) (
	*contestService,
	*contestMocks.MockContestRepository,
	*contestMocks.MockRegistrationRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	contestRepo := contestMocks.NewMockContestRepository(ctrl)
	regRepo := contestMocks.NewMockRegistrationRepository(ctrl)

	svc := NewContestService(contestRepo, regRepo).(*contestService)
	return svc, contestRepo, regRepo
}

// --- Create ---

func TestCreateContest_Success(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	start := time.Now().Add(24 * time.Hour)
	end := time.Now().Add(48 * time.Hour)

	contestRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, c *entity.Contest) error {
			c.ID = 1
			return nil
		},
	)

	contestRepo.EXPECT().AddProblems(gomock.Any(), 1, []int{10, 20}).Return(nil)

	contestRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Contest{
		ID:     1,
		Title:  "Contest 1",
		Status: constant.StatusDraft,
	}, nil)

	contestRepo.EXPECT().CountRegistrations(gomock.Any(), 1).Return(0, nil)
	contestRepo.EXPECT().GetProblemIDs(gomock.Any(), 1).Return([]int{10, 20}, nil)

	resp, err := svc.Create(ctx, 1, &dto.CreateContestRequest{
		Title:      "Contest 1",
		StartTime:  start,
		EndTime:    end,
		ProblemIDs: []int{10, 20},
	})

	assert.NoError(t, err)
	assert.Equal(t, "Contest 1", resp.Title)
	assert.Equal(t, constant.StatusDraft, resp.Status)
}

func TestCreateContest_InvalidTimeRange(t *testing.T) {
	svc, _, _ := newContestTestEnv(t)
	ctx := context.Background()

	start := time.Now().Add(48 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	resp, err := svc.Create(ctx, 1, &dto.CreateContestRequest{
		Title:     "Bad Contest",
		StartTime: start,
		EndTime:   end,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}

// --- Get ---

func TestGetContest_Success(t *testing.T) {
	svc, contestRepo, regRepo := newContestTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:     1,
		Title:  "Contest 1",
		Status: constant.StatusUpcoming,
	}, nil)

	contestRepo.EXPECT().CountRegistrations(ctx, 1).Return(5, nil)
	contestRepo.EXPECT().GetProblemIDs(ctx, 1).Return([]int{10, 20}, nil)

	regRepo.EXPECT().Exists(ctx, 1, 42).Return(true, nil)

	resp, err := svc.Get(ctx, 1, 42)

	assert.NoError(t, err)
	assert.Equal(t, "Contest 1", resp.Title)
	assert.NotNil(t, resp.IsRegistered)
	assert.True(t, *resp.IsRegistered)
}

func TestGetContest_NotFound(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "contest not found", nil))

	resp, err := svc.Get(ctx, 999, 0)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- Delete ---

func TestDeleteContest_Success(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:    1,
		Title: "Contest 1",
	}, nil)

	contestRepo.EXPECT().Delete(ctx, 1).Return(nil)

	err := svc.Delete(ctx, 1)
	assert.NoError(t, err)
}

func TestDeleteContest_NotFound(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	contestRepo.EXPECT().Get(ctx, 999).Return(nil, apperr.New(response.CodeNotFound, "contest not found", nil))

	err := svc.Delete(ctx, 999)
	assert.Error(t, err)
}

// --- Find ---

func TestFindContests_EmptyResult(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	records := []*entity.Contest{}
	contestRepo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.Contest]{
		Records:    &records,
		Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)

	resp, err := svc.Find(ctx, &d.QueryOptions{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

// --- Update ---

func TestUpdateContest_InvalidTimeRange(t *testing.T) {
	svc, contestRepo, _ := newContestTestEnv(t)
	ctx := context.Background()

	start := time.Now().Add(72 * time.Hour)
	end := time.Now().Add(48 * time.Hour)

	contestRepo.EXPECT().Get(ctx, 1).Return(&entity.Contest{
		ID:        1,
		Title:     "Contest 1",
		StartTime: time.Now().Add(24 * time.Hour),
		EndTime:   time.Now().Add(96 * time.Hour),
	}, nil)

	resp, err := svc.Update(ctx, 1, &dto.UpdateContestRequest{
		StartTime: &start,
		EndTime:   &end,
	})

	assert.Nil(t, resp)
	assert.Error(t, err)

	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeBadRequest, appErr.Code)
}
