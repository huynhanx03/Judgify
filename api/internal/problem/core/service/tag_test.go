package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/problem/constant"
	problemDTO "github.com/huynhanx03/judgify/internal/problem/core/dto"
	"github.com/huynhanx03/judgify/internal/problem/core/entity"
	problemMocks "github.com/huynhanx03/judgify/internal/problem/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newTagTestEnv(t *testing.T) (*tagService, *problemMocks.MockTagRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := problemMocks.NewMockTagRepository(ctrl)
	svc := NewTagService(repo).(*tagService)
	return svc, repo
}

func TestFindTags_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	records := []*entity.Tag{{ID: 1, Name: "dp"}}
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
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Tag{ID: 1, Name: "dp"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "dp", resp.Name)
}

func TestGetTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.Tag) error { e.ID = 1; return nil })
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Tag{ID: 1, Name: "dp"}, nil)
	resp, err := svc.Create(context.Background(), &problemDTO.CreateTagRequest{Name: "dp", ElementIDs: []int{1, 2}})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateTag_RepoError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &problemDTO.CreateTagRequest{Name: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Tag{ID: 1, Name: "old"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.Tag{ID: 1, Name: "new"}, nil)
	newName := "new"
	resp, err := svc.Update(context.Background(), 1, &problemDTO.UpdateTagRequest{Name: &newName})
	assert.NoError(t, err)
	assert.Equal(t, "new", resp.Name)
}

func TestUpdateTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &problemDTO.UpdateTagRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteTag_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteTag_NotFound(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjTag)
}

func TestDeleteTag_ExistsError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}

func TestFindAllTags_Success(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Tag{{ID: 1}, {ID: 2}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllTags_RepoError(t *testing.T) {
	svc, repo := newTagTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}
