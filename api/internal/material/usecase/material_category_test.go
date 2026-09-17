package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	materialMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newMatCatTestEnv(t *testing.T) (*materialCategoryService, *materialMocks.MockMaterialCategoryRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := materialMocks.NewMockMaterialCategoryRepository(ctrl)
	caches := appcache.New(testutil.NewCache())
	svc := NewMaterialCategoryService(repo, caches.MaterialCategory).(*materialCategoryService)
	return svc, repo
}

func TestFindAllCategories_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.MaterialCategory{{ID: testutil.ID(1), Name: "Go"}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestFindAllCategories_RepoError(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindCategories_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	records := []*entity.MaterialCategory{{ID: testutil.ID(1), Name: "Go"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.MaterialCategory]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestGetCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.MaterialCategory{ID: testutil.ID(1), Name: "Go"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "Go", resp.Name)
}

func TestGetCategory_NotFound(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.MaterialCategory) error { e.ID = testutil.ID(1); return nil })
	resp, err := svc.Create(context.Background(), &contract.CreateMaterialCategoryCommand{Name: "Go"})
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateCategory_RepoError(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &contract.CreateMaterialCategoryCommand{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.MaterialCategory{ID: testutil.ID(1), Name: "Go"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Rust"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &contract.UpdateMaterialCategoryCommand{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Rust", resp.Name)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), testutil.ID(999), &contract.UpdateMaterialCategoryCommand{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteCategory_Success(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteCategory_Error(t *testing.T) {
	svc, repo := newMatCatTestEnv(t)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(999)).Return(apperr.New(apperr.CodeNotFound, "not found", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(999)))
}
