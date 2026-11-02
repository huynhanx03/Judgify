package domain

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func TestBuildRewardSnapshotRejectsCallerControlledOrInvalidFacts(t *testing.T) {
	t.Parallel()

	validIdentity, validRevision, validFacts := validRewardFactoryInput()
	tests := map[string]func(*RewardMappingIdentity, *int64, *RewardFacts){
		"nil tag mapping ID": func(identity *RewardMappingIdentity, _ *int64, _ *RewardFacts) {
			identity.TagMappingID = uuid.Nil
		},
		"nil element mapping ID": func(identity *RewardMappingIdentity, _ *int64, _ *RewardFacts) {
			identity.ElementMappingID = uuid.Nil
		},
		"zero mapping revision": func(_ *RewardMappingIdentity, revision *int64, _ *RewardFacts) {
			*revision = 0
		},
		"excessive mapping revision": func(_ *RewardMappingIdentity, revision *int64, _ *RewardFacts) {
			*revision = problemcontract.MaximumRewardMappingRevision + 1
		},
		"nil difficulty ID": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Difficulty.ID = uuid.Nil
		},
		"zero difficulty revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Difficulty.Revision = 0
		},
		"excessive difficulty revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Difficulty.Revision = problemcontract.MaximumDifficultyRevision + 1
		},
		"negative base experience": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Difficulty.BaseExperience = -1
		},
		"excessive base experience": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Difficulty.BaseExperience = problemcontract.MaximumBaseExperience + 1
		},
		"nil tag ID": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Tags[0].ID = uuid.Nil
		},
		"zero tag revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Tags[0].Revision = 0
		},
		"excessive tag revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Tags[0].Revision = problemcontract.MaximumRewardMappingRevision + 1
		},
		"duplicate tag fact": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Tags = append(facts.Tags, facts.Tags[0])
		},
		"excessive tag facts": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Tags = make([]TaxonomyRevisionFact, problemcontract.MaximumTags+1)
			for index := range facts.Tags {
				facts.Tags[index] = TaxonomyRevisionFact{
					ID:       rewardFactoryUUID(uint64(100 + index)),
					Revision: 1,
				}
			}
		},
		"nil element ID": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Elements[0].ID = uuid.Nil
		},
		"zero element revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Elements[0].Revision = 0
		},
		"excessive element revision": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Elements[0].Revision = problemcontract.MaximumRewardMappingRevision + 1
		},
		"duplicate element fact": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Elements = append(facts.Elements, facts.Elements[0])
		},
		"excessive element facts": func(_ *RewardMappingIdentity, _ *int64, facts *RewardFacts) {
			facts.Elements = make([]TaxonomyRevisionFact, problemcontract.MaximumElements+1)
			for index := range facts.Elements {
				facts.Elements[index] = TaxonomyRevisionFact{
					ID:       rewardFactoryUUID(uint64(200 + index)),
					Revision: 1,
				}
			}
		},
	}

	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			identity := validIdentity
			revision := validRevision
			facts := cloneRewardFacts(validFacts)
			mutate(&identity, &revision, &facts)

			snapshot, err := BuildRewardSnapshot(identity, revision, facts)
			require.Error(t, err)
			require.Equal(t, RewardSnapshot{}, snapshot)
		})
	}
}

func TestBuildRewardSnapshotIsOrderIndependentAndDeterministic(t *testing.T) {
	t.Parallel()

	identity, revision, facts := validRewardFactoryInput()
	reversed := cloneRewardFacts(facts)
	slices.Reverse(reversed.Tags)
	slices.Reverse(reversed.Elements)

	first, err := BuildRewardSnapshot(identity, revision, facts)
	require.NoError(t, err)
	second, err := BuildRewardSnapshot(identity, revision, reversed)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, []TagSnapshotItem{
		{TagID: rewardFactoryUUID(10), TagRevision: 3},
		{TagID: rewardFactoryUUID(20), TagRevision: 4},
	}, first.Tags.Items)
	require.Equal(t, []ElementSnapshotItem{
		{ElementID: rewardFactoryUUID(30), ElementRevision: 5, WeightBasisPoints: 5_000},
		{ElementID: rewardFactoryUUID(40), ElementRevision: 6, WeightBasisPoints: 5_000},
	}, first.Elements.Items)
	require.NotEqual(t, [32]byte{}, first.Checksum)
	require.NoError(t, first.Validate())
}

func TestBuildRewardSnapshotDeduplicatesElementUnionAndAllocatesExactlyTenThousandBasisPoints(t *testing.T) {
	t.Parallel()

	identity, revision, facts := validRewardFactoryInput()
	facts.Elements = []TaxonomyRevisionFact{
		{ID: rewardFactoryUUID(50), Revision: 8},
		{ID: rewardFactoryUUID(30), Revision: 6},
		{ID: rewardFactoryUUID(40), Revision: 7},
	}

	snapshot, err := BuildRewardSnapshot(identity, revision, facts)
	require.NoError(t, err)
	require.Equal(t, []ElementSnapshotItem{
		{ElementID: rewardFactoryUUID(30), ElementRevision: 6, WeightBasisPoints: 3_334},
		{ElementID: rewardFactoryUUID(40), ElementRevision: 7, WeightBasisPoints: 3_333},
		{ElementID: rewardFactoryUUID(50), ElementRevision: 8, WeightBasisPoints: 3_333},
	}, snapshot.Elements.Items)

	total := 0
	for _, item := range snapshot.Elements.Items {
		total += item.WeightBasisPoints
	}
	require.Equal(t, problemcontract.MaximumRewardWeightBasisPoints, total)
}

func TestBuildRewardSnapshotUsesServerOwnedMappingIdentityAndRevision(t *testing.T) {
	t.Parallel()

	identity, revision, facts := validRewardFactoryInput()
	snapshot, err := BuildRewardSnapshot(identity, revision, facts)
	require.NoError(t, err)

	require.Equal(t, identity.TagMappingID, snapshot.Tags.MappingID)
	require.Equal(t, identity.ElementMappingID, snapshot.Elements.MappingID)
	require.Equal(t, revision, snapshot.Tags.Revision)
	require.Equal(t, revision, snapshot.Elements.Revision)
	require.Equal(t, facts.Difficulty.ID, snapshot.DifficultyID)
	require.Equal(t, facts.Difficulty.Revision, snapshot.DifficultyRevision)
	require.Equal(t, facts.Difficulty.BaseExperience, snapshot.BaseExperience)
	require.NoError(t, snapshot.Validate())

	facts.Elements = nil
	withoutElements, err := BuildRewardSnapshot(identity, revision, facts)
	require.NoError(t, err)
	require.Nil(t, withoutElements.Elements)
	require.NoError(t, withoutElements.Validate())
}

func validRewardFactoryInput() (RewardMappingIdentity, int64, RewardFacts) {
	return RewardMappingIdentity{
			TagMappingID:     rewardFactoryUUID(1),
			ElementMappingID: rewardFactoryUUID(2),
		},
		9,
		RewardFacts{
			Difficulty: DifficultyRewardFact{
				ID:             rewardFactoryUUID(3),
				Revision:       2,
				BaseExperience: 750,
			},
			Tags: []TaxonomyRevisionFact{
				{ID: rewardFactoryUUID(20), Revision: 4},
				{ID: rewardFactoryUUID(10), Revision: 3},
			},
			Elements: []TaxonomyRevisionFact{
				{ID: rewardFactoryUUID(40), Revision: 6},
				{ID: rewardFactoryUUID(30), Revision: 5},
			},
		}
}

func cloneRewardFacts(value RewardFacts) RewardFacts {
	value.Tags = append([]TaxonomyRevisionFact(nil), value.Tags...)
	value.Elements = append([]TaxonomyRevisionFact(nil), value.Elements...)
	return value
}

func rewardFactoryUUID(value uint64) uuid.UUID {
	return uuid.UUID{
		byte(value >> 56), byte(value >> 48), byte(value >> 40), byte(value >> 32),
		byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value),
		0, 0, 0, 0, 0, 0, 0, 1,
	}
}
