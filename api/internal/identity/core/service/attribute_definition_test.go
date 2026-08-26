package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/identity/constant"
	identityDTO "github.com/huynhanx03/judgify/internal/identity/core/dto"
	"github.com/huynhanx03/judgify/internal/identity/core/entity"
	identityMocks "github.com/huynhanx03/judgify/internal/identity/mocks"
	"github.com/huynhanx03/judgify/pkg/common/apperr"
	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/common/cache/ember"
	"github.com/huynhanx03/judgify/pkg/common/http/response"
	d "github.com/huynhanx03/judgify/pkg/dto"
)

func newAttrDefTestEnv(t *testing.T) (*attributeDefinitionService, *identityMocks.MockAttributeDefinitionRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockAttributeDefinitionRepository(ctrl)
	c := ember.New[string, any]()
	svc := NewAttributeDefinitionService(repo, c).(*attributeDefinitionService)
	return svc, repo
}

func TestFindAttributeDefinitions_Success(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	records := []*entity.AttributeDefinition{{ID: 1, Key: "avatar", DataType: "string"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.AttributeDefinition]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindAttributeDefinitions_NilRecords(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.AttributeDefinition]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindAttributeDefinitions_RepoError(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetAttributeDefinition_CacheMiss(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.AttributeDefinition{ID: 1, Key: "avatar"}, nil)
	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "avatar", resp.Key)
}

func TestGetAttributeDefinition_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockAttributeDefinitionRepository(ctrl)
	c := ember.New[string, any]()
	svc := NewAttributeDefinitionService(repo, c).(*attributeDefinitionService)

	// Pre-populate cache
	cache.Set(c, constant.CacheKeyPrefixAttrID+"1", &entity.AttributeDefinition{ID: 1, Key: "cached"})

	resp, err := svc.Get(context.Background(), 1)
	assert.NoError(t, err)
	assert.Equal(t, "cached", resp.Key)
}

func TestGetAttributeDefinition_NotFound(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), 999)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestCreateAttributeDefinition_Success(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.AttributeDefinition) error { e.ID = 1; return nil })
	resp, err := svc.Create(context.Background(), &identityDTO.CreateAttributeDefinitionRequest{Key: "avatar", DataType: "string"})
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.ID)
}

func TestCreateAttributeDefinition_RepoError(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(apperr.New(response.CodeDatabaseError, "err", nil))
	resp, err := svc.Create(context.Background(), &identityDTO.CreateAttributeDefinitionRequest{Key: "x"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateAttributeDefinition_Success(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 1).Return(&entity.AttributeDefinition{ID: 1, Key: "avatar"}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	newKey := "photo"
	resp, err := svc.Update(context.Background(), 1, &identityDTO.UpdateAttributeDefinitionRequest{Key: &newKey})
	assert.NoError(t, err)
	assert.Equal(t, "photo", resp.Key)
}

func TestUpdateAttributeDefinition_NotFound(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), 999).Return(nil, apperr.New(response.CodeNotFound, "not found", nil))
	resp, err := svc.Update(context.Background(), 999, &identityDTO.UpdateAttributeDefinitionRequest{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestDeleteAttributeDefinition_Success(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(true, nil)
	repo.EXPECT().Delete(gomock.Any(), 1).Return(nil)
	assert.NoError(t, svc.Delete(context.Background(), 1))
}

func TestDeleteAttributeDefinition_NotFound(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 999).Return(false, nil)
	err := svc.Delete(context.Background(), 999)
	assert.Error(t, err)
	appErr, _ := err.(*apperr.AppError)
	assert.Equal(t, response.CodeNotFound, appErr.Code)
	assert.Contains(t, appErr.Message, constant.ObjAttributeDefinition)
}

func TestDeleteAttributeDefinition_ExistsError(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Exists(gomock.Any(), 1).Return(false, apperr.New(response.CodeDatabaseError, "err", nil))
	assert.Error(t, svc.Delete(context.Background(), 1))
}
