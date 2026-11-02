package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/go-common/pkg/common/apperr"
	d "github.com/huynhanx03/go-common/pkg/dto"

	appcache "github.com/huynhanx03/judgify/internal/cache"
	cultivationDTO "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"github.com/huynhanx03/judgify/internal/testutil"
	cultMocks "github.com/huynhanx03/judgify/internal/testutil/mocks"
)

func newTraitTestEnv(t *testing.T) (*traitService, *cultMocks.MockTraitRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockTraitRepository(ctrl)
	caches := appcache.New(testutil.NewCache())
	svc := NewTraitService(repo, nil, caches.CultivationTrait, nil).(*traitService)
	return svc, repo
}

func TestFindTraits_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	records := []*entity.Trait{{ID: testutil.ID(1), Name: "Sharp Eye"}}
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Trait]{Records: &records, Pagination: d.CalculatePagination(1, 10, 1)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Len(t, *resp.Records, 1)
}

func TestFindTraits_NilRecords(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(&d.Paginated[*entity.Trait]{Records: nil, Pagination: d.CalculatePagination(1, 10, 0)}, nil)
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.NoError(t, err)
	assert.Empty(t, *resp.Records)
}

func TestFindTraits_RepoError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.Find(context.Background(), &d.QueryOptions{})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestFindAllTraits_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return([]*entity.Trait{{ID: testutil.ID(1)}, {ID: testutil.ID(2)}}, nil)
	resp, err := svc.FindAll(context.Background())
	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestFindAllTraits_RepoError(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().FindAll(gomock.Any()).Return(nil, apperr.New(apperr.CodeDatabaseError, "err", nil))
	resp, err := svc.FindAll(context.Background())
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestGetTrait_Success(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).Return(&entity.Trait{ID: testutil.ID(1), Name: "Sharp Eye"}, nil)
	resp, err := svc.Get(context.Background(), testutil.ID(1))
	assert.NoError(t, err)
	assert.Equal(t, "Sharp Eye", resp.Name)
}

func TestGetTrait_NotFound(t *testing.T) {
	svc, repo := newTraitTestEnv(t)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(999)).Return(nil, apperr.New(apperr.CodeNotFound, "not found", nil))
	resp, err := svc.Get(context.Background(), testutil.ID(999))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

type traitEffectRecorder struct {
	events  *[]string
	lockErr error
	current entity.TraitEffectRevision
}

func (recorder *traitEffectRecorder) LockTimeline(context.Context) error {
	*recorder.events = append(*recorder.events, "lock_timeline")
	return recorder.lockErr
}

func (recorder *traitEffectRecorder) Publish(
	_ context.Context,
	traitID uuid.UUID,
	revision int64,
	effect entity.TraitEffectSpec,
) (entity.TraitEffectRevision, error) {
	*recorder.events = append(*recorder.events, "publish_effect")
	published, err := entity.NewTraitEffectRevision(
		uuid.New(), traitID, revision, effect, nil,
		time.Date(2026, 8, 9, 2, 0, 0, 0, time.UTC),
	)
	recorder.current = published
	return published, err
}

func (recorder *traitEffectRecorder) Current(
	context.Context,
	uuid.UUID,
) (entity.TraitEffectRevision, error) {
	return recorder.current, nil
}

func (recorder *traitEffectRecorder) EffectiveAt(
	context.Context,
	uuid.UUID,
	time.Time,
) (entity.TraitEffectRevision, error) {
	return recorder.current, nil
}

type traitCatalogRecorder struct{ events *[]string }

func (recorder traitCatalogRecorder) InvalidatePool(context.Context) error {
	*recorder.events = append(*recorder.events, "invalidate_catalog")
	return nil
}

func TestCreateTraitLocksRewardTimelineBeforeWritingCatalog(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockTraitRepository(ctrl)
	events := make([]string, 0, 5)
	effects := &traitEffectRecorder{events: &events}
	svc := NewTraitService(
		repo,
		traitCatalogRecorder{events: &events},
		appcache.New(testutil.NewCache()).CultivationTrait,
		effects,
	).(*traitService)

	var created *entity.Trait
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, value *entity.Trait) error {
			events = append(events, "create_trait")
			value.ID = testutil.ID(1)
			value.Version = 1
			created = value
			return nil
		},
	)
	repo.EXPECT().Get(gomock.Any(), testutil.ID(1)).DoAndReturn(
		func(context.Context, uuid.UUID) (*entity.Trait, error) {
			events = append(events, "read_trait")
			value := *created
			value.EffectRevision = &effects.current
			return &value, nil
		},
	)

	bonus := int64(25)
	response, err := svc.Create(context.Background(), &cultivationDTO.CreateTraitCommand{
		Code: "sharp_eye", Type: entity.TraitTypeTalent, Name: "Sharp Eye",
		RarityID: testutil.ID(5), DisplayOrder: 10,
		Effect: entity.TraitEffectSpec{
			Version:   entity.TraitEffectSchemaVersion,
			Kind:      entity.TraitEffectExperienceBonus,
			Scope:     entity.TraitEffectScopeAll,
			FlatBonus: &bonus,
		},
		Reason: "publish initial reward effect",
	})

	require.NoError(t, err)
	require.Equal(t, testutil.ID(1), response.ID)
	require.Equal(t, []string{
		"lock_timeline",
		"create_trait",
		"publish_effect",
		"read_trait",
		"invalidate_catalog",
	}, events)
}

func TestCreateTraitStopsBeforeCatalogWriteWhenTimelineLockFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := cultMocks.NewMockTraitRepository(ctrl)
	events := make([]string, 0, 1)
	effects := &traitEffectRecorder{
		events:  &events,
		lockErr: apperr.New(apperr.CodeDatabaseError, "lock failed", nil),
	}
	svc := NewTraitService(
		repo,
		traitCatalogRecorder{events: &events},
		appcache.New(testutil.NewCache()).CultivationTrait,
		effects,
	).(*traitService)
	bonus := int64(25)

	response, err := svc.Create(context.Background(), &cultivationDTO.CreateTraitCommand{
		Code: "sharp_eye", Type: entity.TraitTypeTalent, Name: "Sharp Eye",
		RarityID: testutil.ID(5),
		Effect: entity.TraitEffectSpec{
			Version:   entity.TraitEffectSchemaVersion,
			Kind:      entity.TraitEffectExperienceBonus,
			Scope:     entity.TraitEffectScopeAll,
			FlatBonus: &bonus,
		},
		Reason: "publish initial reward effect",
	})

	require.Nil(t, response)
	require.Error(t, err)
	require.Equal(t, []string{"lock_timeline"}, events)
}
