package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	entity "github.com/huynhanx03/judgify/internal/identity/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	identityMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newAttrDefTestEnv(t *testing.T) (*attributeDefinitionService, *identityMocks.MockAttributeDefinitionRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockAttributeDefinitionRepository(ctrl)
	c := testutil.NewCache()
	svc := NewAttributeDefinitionService(repo, appcache.New(c).IdentityAttributeDefinition).(*attributeDefinitionService)
	return svc, repo
}

func TestFindAttributeDefinitions_Success(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	records := []*entity.AttributeDefinition{{ID: testutil.ID(1), Key: "avatar"}}
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
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetAttributeDefinition_CacheMiss(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.AttributeDefinition{ID: testutil.ID(1), Key: "avatar"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "avatar", resp.Key)
}

func TestGetAttributeDefinition_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := identityMocks.NewMockAttributeDefinitionRepository(ctrl)
	c := testutil.NewCache()
	svc := NewAttributeDefinitionService(repo, appcache.New(c).IdentityAttributeDefinition).(*attributeDefinitionService)

	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(
		&entity.AttributeDefinition{ID: testutil.ID(1), Key: "cached"}, nil,
	).Times(1)
	_, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "cached", resp.Key)
}

func TestGetAttributeDefinition_NotFound(t *testing.T) {
	svc, repo := newAttrDefTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}
