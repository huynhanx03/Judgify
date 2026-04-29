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
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newDiffTestEnv(t *testing.T) (*difficultyService, *problemMocks.MockDifficultyRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := problemMocks.NewMockDifficultyRepository(ctrl)
	svc := NewDifficultyService(repo).(*difficultyService)
	return svc, repo
}

func TestFindDifficulties_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	records := []*entity.Difficulty{{ID: 1, Name: "Easy", Level: 1}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Difficulty]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindDifficulties_NilRecords(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Difficulty]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindDifficulties_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Difficulty{ID: 1, Name: "Easy"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Easy", resp.Name)
}

func TestGetDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Difficulty) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &problemDTO.CreateDifficultyRequest{Name: "Easy", Level: 1})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateDifficulty_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &problemDTO.CreateDifficultyRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Difficulty{ID: 1, Name: "Easy"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Medium"
	resp, err := svc.Update(context.Background(), 1, &problemDTO.UpdateDifficultyRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Medium", resp.Name)
}

func TestUpdateDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &problemDTO.UpdateDifficultyRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateDifficulty_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Difficulty{ID: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &problemDTO.UpdateDifficultyRequest{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjDifficulty)
}

func TestDeleteDifficulty_ExistsError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllDifficulties_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Difficulty{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllDifficulties_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
