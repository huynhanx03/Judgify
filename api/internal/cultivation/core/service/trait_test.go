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

func newTraitTestEnv(t *testing.T) (*traitService, *cultMocks.MockTraitRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockTraitRepository(ctrl)
	svc := NewTraitService(repo, nil).(*traitService) // gachaService nil
	return svc, repo
}

func TestFindTraits_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	records := []*entity.Trait{{ID: 1, Name: "Sharp Eye"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Trait]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindTraits_NilRecords(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Trait]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindTraits_RepoError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindAllTraits_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Trait{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllTraits_RepoError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTrait_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Trait{ID: 1, Name: "Sharp Eye"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Sharp Eye", resp.Name)
}

func TestGetTrait_NotFound(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTrait_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Trait) error { e.ID = 1; return nil })
	// Create re-fetches to get full rarity data
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Trait{ID: 1, Name: "Sharp Eye", RarityID: 5}, nil)
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateTraitRequest{
		Type: "talent", Name: "Sharp Eye", RarityID: 5,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateTrait_RepoCreateError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateTraitRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTrait_RefetchError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Trait) error { e.ID = 1; return nil })
	repo.EXPECT().Get(gomock.Any(), 1).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateTraitRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTrait_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Trait{ID: 1, Name: "Old"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	// Update re-fetches
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Trait{ID: 1, Name: "New"}, nil)
	newName := "New"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateTraitRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "New", resp.Name)
}

func TestUpdateTrait_NotFound(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &cultivationDTO.UpdateTraitRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTrait_RepoUpdateError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Trait{ID: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateTraitRequest{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteTrait_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteTrait_NotFound(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjTrait)
}

func TestDeleteTrait_ExistsError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestDeleteTrait_DeleteError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}
