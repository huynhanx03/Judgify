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

func newRarityTestEnv(t *testing.T) (*rarityService, *cultMocks.MockRarityRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockRarityRepository(ctrl)
	// gachaService is nil — pool invalidation is skipped
	svc := NewRarityService(repo, nil).(*rarityService)
	return svc, repo
}

func TestFindRarities_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	records := []*entity.Rarity{{ID: 1, Name: "Common", Code: "common", Weight: 50}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Rarity]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindRarities_NilRecords(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Rarity]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindRarities_RepoError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Rarity{ID: 1, Name: "Common"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Common", resp.Name)
}

func TestGetRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Rarity) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRarityRequest{Code: "common", Weight: 50})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateRarity_RepoError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRarityRequest{Code: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Rarity{ID: 1, Name: "Common"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Uncommon"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateRarityRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Uncommon", resp.Name)
}

func TestUpdateRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &cultivationDTO.UpdateRarityRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjRarity)
}

func TestDeleteRarity_ExistsError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestDeleteRarity_DeleteError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllRarities_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Rarity{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllRarities_RepoError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
