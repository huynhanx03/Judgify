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

func newElementTestEnv(t *testing.T) (*elementService, *cultMocks.MockElementRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockElementRepository(ctrl)
	svc := NewElementService(repo).(*elementService)
	return svc, repo
}

func TestFindElements_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	records := []*entity.Element{{ID: 1, Name: "Fire", Code: "fire"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Element]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindElements_NilRecords(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Element]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindElements_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Element{ID: 1, Name: "Fire"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Fire", resp.Name)
}

func TestGetElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Element) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateElementRequest{Name: "Fire", Code: "fire"})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateElement_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateElementRequest{Name: "Fire"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Element{ID: 1, Name: "Fire"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Water"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateElementRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Water", resp.Name)
}

func TestUpdateElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &cultivationDTO.UpdateElementRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateElement_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Element{ID: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), 1, &cultivationDTO.UpdateElementRequest{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjElement)
}

func TestDeleteElement_ExistsError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestDeleteElement_DeleteError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllElements_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Element{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllElements_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
