package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	problemDTO "github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	problemMocks "github.com/huynhanx03/judgify/internal/problem/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
)

func newTCTestEnv(t *testing.T) (*testCaseService, *problemMocks.MockTestCaseRepository, *problemMocks.MockProblemRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	tcRepo := problemMocks.NewMockTestCaseRepository(ctrl)
	probRepo := problemMocks.NewMockProblemRepository(ctrl)
	svc := NewTestCaseService(tcRepo, probRepo).(*testCaseService)
	return svc, tcRepo, probRepo
}

func TestFindByProblemID_Success(t *testing.T) {
	svc, tcRepo, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 1).Return(true, nil)
	tcRepo.EXPECT().FindByProblemID(ctx, 1).Return([]*entity.TestCase{{ID: 1, ProblemID: 1}}, nil)
	resp, err := svc.FindByProblemID(ctx, 1)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindByProblemID_ProblemNotFound(t *testing.T) {
	svc, _, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 999).Return(false, nil)
	resp, err := svc.FindByProblemID(ctx, 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjProblem)
}

func TestFindByProblemID_RepoError(t *testing.T) {
	svc, _, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindByProblemID(ctx, 1)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTestCase_Success(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.TestCase{ID: 1, Input: "1 2", ExpectedOutput: "3"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "1 2", resp.Input)
}

func TestGetTestCase_NotFound(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTestCase_Success(t *testing.T) {
	svc, tcRepo, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 1).Return(true, nil)
	tcRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, tc *entity.TestCase) error { tc.ID = 1; return nil })
	resp, err := svc.Create(ctx, &problemDTO.CreateTestCaseRequest{ProblemID: 1, Input: "1 2", ExpectedOutput: "3"})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateTestCase_ProblemNotFound(t *testing.T) {
	svc, _, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 999).Return(false, nil)
	resp, err := svc.Create(ctx, &problemDTO.CreateTestCaseRequest{ProblemID: 999})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTestCase_RepoError(t *testing.T) {
	svc, tcRepo, probRepo := newTCTestEnv(t)
	ctx := context.Background()
	probRepo.EXPECT().Exists(ctx, 1).Return(true, nil)
	tcRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(ctx, &problemDTO.CreateTestCaseRequest{ProblemID: 1})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTestCase_Success(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.TestCase{ID: 1, Input: "old"}, nil)
	tcRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newInput := "new"
	resp, err := svc.Update(context.Background(), 1, &problemDTO.UpdateTestCaseRequest{Input: &newInput})
	assert.NoError(t, err)
	assert.Equal(t, "new", resp.Input)
}

func TestUpdateTestCase_NotFound(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &problemDTO.UpdateTestCaseRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTestCase_RepoError(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Get(gomock.Any(), 1).Return(&entity.TestCase{ID: 1}, nil)
	tcRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &problemDTO.UpdateTestCaseRequest{Input: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteTestCase_Success(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	tcRepo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteTestCase_NotFound(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjTestCase)
}

func TestDeleteTestCase_ExistsError(t *testing.T) {
	svc, tcRepo, _ := newTCTestEnv(t)
	tcRepo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}
