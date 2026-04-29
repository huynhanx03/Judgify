package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/huynhanx03/judgify/internal/cultivation/core/entity"
	cultMocks "github.com/huynhanx03/judgify/internal/cultivation/mocks"
)

func newGachaTestEnv(t *testing.T) (
	*gachaService,
	*cultMocks.MockTraitRepository,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	traitRepo := cultMocks.NewMockTraitRepository(ctrl)
	svc := NewGachaService(traitRepo).(*gachaService)
	return svc, traitRepo
}

func sampleTraits() []*entity.TraitWithWeight {
	return []*entity.TraitWithWeight{
		{Trait: &entity.Trait{ID: 1, Type: "root_bone", Name: "Dragon Bone"}, Weight: 10},
		{Trait: &entity.Trait{ID: 2, Type: "root_bone", Name: "Phoenix Bone"}, Weight: 5},
		{Trait: &entity.Trait{ID: 3, Type: "talent", Name: "Fire Talent"}, Weight: 20},
		{Trait: &entity.Trait{ID: 4, Type: "talent", Name: "Water Talent"}, Weight: 15},
		{Trait: &entity.Trait{ID: 5, Type: "talent", Name: "Earth Talent"}, Weight: 10},
		{Trait: &entity.Trait{ID: 6, Type: "talent", Name: "Wind Talent"}, Weight: 10},
		{Trait: &entity.Trait{ID: 7, Type: "talent", Name: "Lightning Talent"}, Weight: 10},
		{Trait: &entity.Trait{ID: 8, Type: "talent", Name: "Ice Talent"}, Weight: 10},
		{Trait: &entity.Trait{ID: 9, Type: "talent", Name: "Dark Talent"}, Weight: 5},
	}
}

// --- LoadPool ---

func TestLoadPool_Success(t *testing.T) {
	svc, traitRepo := newGachaTestEnv(t)
	ctx := context.Background()

	traitRepo.EXPECT().FindAllWithWeight(ctx).Return(sampleTraits(), nil)

	err := svc.LoadPool(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, svc.rootBonePool)
	assert.NotNil(t, svc.talentPool)
	assert.Equal(t, 2, svc.rootBonePool.size())
	assert.Equal(t, 7, svc.talentPool.size())
}

func TestLoadPool_RepoError(t *testing.T) {
	svc, traitRepo := newGachaTestEnv(t)
	ctx := context.Background()

	traitRepo.EXPECT().FindAllWithWeight(ctx).Return(nil, assert.AnError)

	err := svc.LoadPool(ctx)
	assert.Error(t, err)
}

// --- Roll ---

func TestRoll_Success(t *testing.T) {
	svc, traitRepo := newGachaTestEnv(t)
	ctx := context.Background()

	traitRepo.EXPECT().FindAllWithWeight(ctx).Return(sampleTraits(), nil)
	err := svc.LoadPool(ctx)
	require.NoError(t, err)

	resp, err := svc.Roll(ctx)

	assert.NoError(t, err)
	assert.Len(t, resp.RootBones, 1) // DefaultRootBoneCount = 1
	assert.Len(t, resp.Talents, 6)   // DefaultTalentCount = 6
	seen := make(map[int]struct{}, len(resp.Talents))
	for _, talent := range resp.Talents {
		_, ok := seen[talent.ID]
		assert.False(t, ok, "duplicate talent rolled")
		seen[talent.ID] = struct{}{}
	}
}

func TestRoll_PoolNotLoaded(t *testing.T) {
	svc, _ := newGachaTestEnv(t)
	ctx := context.Background()

	resp, err := svc.Roll(ctx)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestRoll_EmptyPool(t *testing.T) {
	svc, traitRepo := newGachaTestEnv(t)
	ctx := context.Background()

	// Return only root_bone traits, no talents — talent pool will be empty
	traitRepo.EXPECT().FindAllWithWeight(ctx).Return([]*entity.TraitWithWeight{
		{Trait: &entity.Trait{ID: 1, Type: "root_bone", Name: "Bone"}, Weight: 10},
	}, nil)

	err := svc.LoadPool(ctx)
	require.NoError(t, err)

	resp, err := svc.Roll(ctx)

	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- InvalidatePool ---

func TestInvalidatePool_Success(t *testing.T) {
	svc, traitRepo := newGachaTestEnv(t)
	ctx := context.Background()

	traitRepo.EXPECT().FindAllWithWeight(ctx).Return(sampleTraits(), nil)

	err := svc.InvalidatePool(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, svc.rootBonePool)
}
