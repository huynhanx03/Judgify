package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	cultivationDTO "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	cultMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newRarityTestEnv(t *testing.T) (*rarityService, *cultMocks.MockRarityRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockRarityRepository(ctrl)
	// Rarity writes change the weighted trait-offer pool, so the catalog
	// invalidator is a required collaborator even in focused unit tests.
	caches := appcache.New(testutil.NewCache())
	svc := NewRarityService(repo, rarityCatalogInvalidator{}, caches.CultivationRarity).(*rarityService)
	return svc, repo
}

type rarityCatalogInvalidator struct{}

func (rarityCatalogInvalidator) InvalidatePool(context.Context) error { return nil }

func TestFindRarities_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	records := []*entity.Rarity{{ID: testutil.ID(1), Name: "Common", Code: "common", Weight: 50}}
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
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Rarity{ID: testutil.ID(1), Name: "Common"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "Common", resp.Name)
}

func TestGetRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Rarity) error { e.ID = testutil.ID(1); return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRarityCommand{Code: "common", Weight: 50})
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateRarity_RepoError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateRarityCommand{Code: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Rarity{ID: testutil.ID(1), Name: "Common"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Uncommon"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &cultivationDTO.UpdateRarityCommand{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Uncommon", resp.Name)
}

func TestUpdateRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), testutil.ID(999), &cultivationDTO.UpdateRarityCommand{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteRarity_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteRarity_NotFound(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(999)).Return(false, nil)
	err := svc.Delete(context.Background(), testutil.ID(999))
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.CultivationObjRarity)
}

func TestDeleteRarity_ExistsError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(false, apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteRarity_DeleteError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestFindAllRarities_Success(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Rarity{{ID: testutil.ID(1)}, {ID: testutil.ID(2)}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllRarities_RepoError(t *testing.T) {
	svc, repo := newRarityTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
