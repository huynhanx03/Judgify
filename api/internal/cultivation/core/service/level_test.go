package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/cultivation/constant"
	cultivationDTO "github.com/huynhanx03/judgify/internal/cultivation/core/dto"
	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newLevelTestEnv(t *testing.T) (*levelService, *cultMocks.MockLevelRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockLevelRepository(ctrl)
	svc := NewLevelService(repo).(*levelService)
	return svc, repo
}

func TestFindLevels_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	records := []*entity.Level{{ID: 1, Name: "Beginner", MinExp: 0}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Level]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindLevels_NilRecords(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Level]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindLevels_RepoError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetLevel_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Level{ID: 1, Name: "Beginner"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Beginner", resp.Name)
}

func TestGetLevel_NotFound(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateLevel_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Level) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateLevelRequest{Name: "Beginner", MinExp: 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateLevel_RepoError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateLevelRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateLevel_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Level{ID: 1, Name: "Beginner", MinExp: 0}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Novice"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateLevelRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Novice", resp.Name)
}

func TestUpdateLevel_NotFound(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &cultivationDTO.UpdateLevelRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateLevel_RepoError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Level{ID: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateLevelRequest{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteLevel_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteLevel_NotFound(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjLevel)
}

func TestDeleteLevel_ExistsError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestDeleteLevel_DeleteError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllLevels_Success(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Level{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllLevels_RepoError(t *testing.T) {
	svc, repo := newLevelTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
