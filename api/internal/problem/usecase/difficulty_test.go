package usecase

import (
	"context"
	"errors"
	"testing"

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

func newDiffTestEnv(t *testing.T) (*difficultyService, *problemMocks.MockDifficultyRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := problemMocks.NewMockDifficultyRepository(ctrl)
	caches := appcache.New(testutil.NewCache())
	svc := NewDifficultyService(repo, caches.ProblemDifficulty).(*difficultyService)
	return svc, repo
}

func TestFindDifficulties_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	records := []*entity.Difficulty{{ID: testutil.ID(1), Name: "Easy", Level: 1}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Difficulty]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindDifficulties_NilRecords(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Difficulty]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindDifficulties_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Difficulty{ID: testutil.ID(1), Name: "Easy"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "Easy", resp.Name)
}

func TestGetDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Difficulty) error { e.ID = testutil.ID(1); return nil })
	resp, err := svc.Create(context.Background(), &problemDTO.CreateDifficultyCommand{Name: "Easy", Level: 1})
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
}

func TestCreateDifficulty_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &problemDTO.CreateDifficultyCommand{Name: "X"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Difficulty{ID: testutil.ID(1), Name: "Easy"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newName := "Medium"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &problemDTO.UpdateDifficultyCommand{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "Medium", resp.Name)
}

func TestUpdateDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), testutil.ID(999), &problemDTO.UpdateDifficultyCommand{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateDifficulty_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Difficulty{ID: testutil.ID(1)}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperr.New(apperr.CodeDatabaseError, "err", nil))
	n := "x"
	resp, err := svc.Update(context.Background(), testutil.ID(1), &problemDTO.UpdateDifficultyCommand{Name: &n})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateDifficultyMapsOnlyVersionConflictToSafeConflict(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Difficulty{ID: testutil.ID(1), Name: "Easy", Version: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(ports.ErrDifficultyVersionConflict)
	name := "Hard"
	response, err := svc.Update(context.Background(), testutil.ID(1), &problemDTO.UpdateDifficultyCommand{Name: &name})
	require.Nil(t, response)
	require.ErrorIs(t, err, ports.ErrDifficultyVersionConflict)
	var appError *apperr.AppError
	require.ErrorAs(t, err, &appError)
	require.Equal(t, apperr.CodeConflict, appError.Code)
	require.Equal(t, map[string]any{"reason": "version_conflict"}, appError.Details)
	require.False(t, errors.Is(apperr.New(apperr.CodeConflict, "other", nil), ports.ErrDifficultyVersionConflict))
}

func TestDeleteDifficulty_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), testutil.ID(1)).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestDeleteDifficulty_NotFound(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(999)).Return(false, nil)
	err := svc.Delete(context.Background(), testutil.ID(999))
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ProblemObjDifficulty)
}

func TestDeleteDifficulty_ExistsError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), testutil.ID(1)).Return(false, apperr.New(apperr.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), testutil.ID(1)))
}

func TestFindAllDifficulties_Success(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Difficulty{{ID: testutil.ID(1)}, {ID: testutil.ID(2)}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllDifficulties_RepoError(t *testing.T) {
	svc, repo := newDiffTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
