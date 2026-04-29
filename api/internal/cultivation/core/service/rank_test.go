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

func newRankTestEnv(t *testing.T) (*rankService, *cultMocks.MockRankRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockRankRepository(ctrl)
	svc := NewRankService(repo).(*rankService)
	return svc, repo
}

func TestFindRanks_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	records := []*entity.Rank{{ID: 1, Name: "Bronze", MinRating: 0}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Rank]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindRanks_NilRecords(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Rank]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindRanks_RepoError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetRank_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Rank{ID: 1, Name: "Bronze"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Bronze", resp.Name)
}

func TestGetRank_NotFound(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateRank_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Rank) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRankRequest{Name: "Bronze", MinRating: 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateRank_RepoError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRankRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateRank_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Rank{ID: 1, Name: "Bronze"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Silver"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateRankRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Silver", resp.Name)
}

func TestUpdateRank_NotFound(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &cultivationDTO.UpdateRankRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateRank_RepoError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Rank{ID: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateRankRequest{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteRank_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteRank_NotFound(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjRank)
}

func TestDeleteRank_ExistsError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestDeleteRank_DeleteError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllRanks_Success(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Rank{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllRanks_RepoError(t *testing.T) {
	svc, repo := newRankTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
