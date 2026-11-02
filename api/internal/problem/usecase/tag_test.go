package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	"github.com/huynhanx03/judgify/internal/constant"
	"github.com/huynhanx03/judgify/internal/ports"
	problemDTO "github.com/huynhanx03/judgify/internal/problem/contract"
	entity "github.com/huynhanx03/judgify/internal/problem/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	problemMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newTagTestEnv(t *testing.T) (*tagService, *problemMocks.MockTagRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := problemMocks.NewMockTagRepository(ctrl)
	caches := appcache.New(testutil.NewCache())
	svc := NewTagService(repo, caches.ProblemTag).(*tagService)
	return svc, repo
}

func TestFindTags_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	records := []*entity.Tag{{ID: testutil.ID(1), Name: "dp"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Tag]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindTags_NilRecords(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Tag]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindTags_RepoError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Tag{ID: testutil.ID(1), Name: "dp"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "dp", resp.Name)
}

func TestGetTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Tag) error { e.ID = testutil.ID(1); return nil })
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Tag{ID: testutil.ID(1), Name: "dp"}, nil)
	resp, err := svc.Create(context.Background(), &problemDTO.CreateTagCommand{Name: "dp", ElementIDs: []uuid.UUID{testutil.ID(1), testutil.ID(2)}})
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateTag_RepoError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &problemDTO.CreateTagCommand{Name: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Tag{ID: testutil.ID(1), Name: "old"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Tag{ID: testutil.ID(1), Name: "new"}, nil)
	newName := "new"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &problemDTO.UpdateTagCommand{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "new", resp.Name)
}

func TestUpdateTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), testutil.ID(999), &problemDTO.UpdateTagCommand{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTagMapsVersionConflictToSafeConflict(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Tag{ID: testutil.ID(1), Name: "old", Version: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(ports.ErrTagVersionConflict)
	name := "new"
	response, err := svc.Update(context.Background(), testutil.ID(1), &problemDTO.UpdateTagCommand{Name: &name})
	require.Nil(t, response)
	require.ErrorIs(t, err, ports.ErrTagVersionConflict)
	var appError *apperr.AppError
	require.ErrorAs(t, err, &appError)
	require.Equal(t, apperr.CodeConflict, appError.Code)
	require.Equal(t, map[string]any{"reason": "version_conflict"}, appError.Details)
}

func TestDeleteTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(999)).Return(false, nil)
	err := svc.Delete(context.Background(), testutil.ID(999))
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ProblemObjTag)
}

func TestDeleteTag_ExistsError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(false, apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestFindAllTags_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Tag{{ID: testutil.ID(1)}, {ID: testutil.ID(2)}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllTags_RepoError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
