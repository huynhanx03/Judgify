package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	cultMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newUserTraitTestEnv(t *testing.T) (
	*userTraitService,
	*cultMocks.MockUserTraitRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockUserTraitRepository(ctrl)
	svc := NewUserTraitService(repo).(*userTraitService)
	return svc, repo
}

func TestFindUserTraits_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()
	records := []*entity.UserTrait{
		{ID: testutil.ID(1), UserID: testutil.ID(10), TraitID: testutil.ID(100)},
		{ID: testutil.ID(2), UserID: testutil.ID(10), TraitID: testutil.ID(200)},
	}
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserTrait]{
		Records: &records, Pagination: d.CalculatePagination(1, 10, 2),
	}, nil)
	resp, err := svc.Find(ctx, &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 2)
}

func TestFindUserTraits_NilRecords(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()
	repo.EXPECT().Find(ctx, gomock.Any()).Return(&d.Paginated[*entity.UserTrait]{
		Records: nil, Pagination: d.CalculatePagination(1, 10, 0),
	}, nil)
	resp, err := svc.Find(ctx, &d.QueryOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, *resp.Records)
}

func TestFindUserTraits_RepoError(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()
	repo.EXPECT().Find(ctx, gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "db error", nil))
	resp, err := svc.Find(ctx, &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetUserTrait_Success(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, testutil.ID(1)).Return(&entity.UserTrait{
		ID: testutil.ID(1), UserID: testutil.ID(10), TraitID: testutil.ID(100),
	}, nil)
	resp, err := svc.Get(ctx, testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, testutil.ID(1), resp.ID)
	assert.Equal(t, testutil.ID(10), resp.UserID)
	assert.Equal(t, testutil.ID(100), resp.TraitID)
}

func TestGetUserTrait_NotFound(t *testing.T) {
	svc, repo := newUserTraitTestEnv(t)
	ctx := context.Background()
	repo.EXPECT().Get(ctx, testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(ctx, testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}
