package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/material/core/dto"
	"github.com/huynhanx03/judgify/internal/material/core/entity"
	materialMocks "github.com/huynhanx03/judgify/internal/material/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newMatCatTestEnv(t *testing.T) (*materialCategoryService, *materialMocks.MockMaterialCategoryRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := materialMocks.NewMockMaterialCategoryRepository(ctrl)
	svc := NewMaterialCategoryService(repo).(*materialCategoryService)
	return svc, repo
}

func TestFindAllCategories_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.MaterialCategory{{ID: 1, Name: "Go"}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindAllCategories_RepoError(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindCategories_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	records := []*entity.MaterialCategory{{ID: 1, Name: "Go"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.MaterialCategory]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestGetCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.MaterialCategory{ID: 1, Name: "Go"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "Go", resp.Name)
}

func TestGetCategory_NotFound(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.MaterialCategory) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &dto.CreateMaterialCategoryRequest{Name: "Go"})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateCategory_RepoError(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &dto.CreateMaterialCategoryRequest{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.MaterialCategory{ID: 1, Name: "Go"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Rust"
	resp, err := svc.Update(context.Background(), 1, &dto.UpdateMaterialCategoryRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Rust", resp.Name)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &dto.UpdateMaterialCategoryRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteCategory_Error(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Delete(gomock.Any(), 999).Return(apperr.New(response.CodeNotFound, "not found", nil))
	assert.Error(t, svc.Delete(context.Background(), 999))
}
