package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	cultivationDTO "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/ports"
	"github.com/huynhanx03/judgify/internal/testutil"
	cultMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newElementTestEnv(t *testing.T) (*elementService, *cultMocks.MockElementRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockElementRepository(ctrl)
	caches := appcache.New(testutil.NewCache())
	svc := NewElementService(repo, caches.CultivationElement).(*elementService)
	return svc, repo
}

func TestFindElements_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	records := []*entity.Element{{ID: testutil.ID(1), Name: "Fire", Code: "fire"}}
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
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Element{ID: testutil.ID(1), Name: "Fire"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "Fire", resp.Name)
}

func TestGetElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Element) error { e.ID = testutil.ID(1); return nil })
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateElementCommand{Name: "Fire", Code: "fire"})
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateElement_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &cultivationDTO.CreateElementCommand{Name: "Fire"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Element{ID: testutil.ID(1), Name: "Fire"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Water"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &cultivationDTO.UpdateElementCommand{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Water", resp.Name)
}

func TestUpdateElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), testutil.ID(999), &cultivationDTO.UpdateElementCommand{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateElement_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Element{ID: testutil.ID(1)}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &cultivationDTO.UpdateElementCommand{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateElementMapsVersionConflictToSafeConflict(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Element{ID: testutil.ID(1), Name: "Fire", Version: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(ports.ErrElementVersionConflict)
	name := "Water"
	response, err := svc.Update(context.Background(), testutil.ID(1), &cultivationDTO.UpdateElementCommand{Name: &name})
	require.Nil(t, response)
	require.ErrorIs(t, err, ports.ErrElementVersionConflict)
	var appError *apperr.AppError
	require.ErrorAs(t, err, &appError)
	require.Equal(t, apperr.CodeConflict, appError.Code)
	require.Equal(t, map[string]any{"reason": "version_conflict"}, appError.Details)
}

func TestDeleteElement_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteElement_NotFound(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(999)).Return(false, nil)
	err := svc.Delete(context.Background(), testutil.ID(999))
	assert.Error(t, err)
	appErr, ok := err.(*apperr.AppError)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.CultivationObjElement)
}

func TestDeleteElement_ExistsError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(false, apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteElement_DeleteError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestFindAllElements_Success(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Element{{ID: testutil.ID(1)}, {ID: testutil.ID(2)}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllElements_RepoError(t *testing.T) {
	svc, repo := newElementTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
