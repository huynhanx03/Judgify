package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserRewardProfileRequiresOneRootAndThreeOrderedTalents(t *testing.T) {
	now := time.Date(2026, 8, 9, 2, 0, 0, 0, time.UTC)
	profile := rewardProfileFixture(t, now)

	require.True(t, profile.Valid())

	missing := profile
	missing.Traits = append([]RewardProfileTrait(nil), profile.Traits[:3]...)
	require.False(t, missing.Valid())

	duplicate := profile
	duplicate.Traits = append([]RewardProfileTrait(nil), profile.Traits...)
	duplicate.Traits[3].TraitID = duplicate.Traits[2].TraitID
	require.False(t, duplicate.Valid())
}

func TestRewardProfileChecksumBindsExactEffectRevisions(t *testing.T) {
	profile := rewardProfileFixture(t, time.Date(2026, 8, 9, 2, 0, 0, 0, time.UTC))
	original := profile.AssignmentChecksum

	current := profile.Traits[1].EffectRevision
	replacement, err := NewTraitEffectRevision(
		uuid.New(),
		current.TraitID,
		current.Revision,
		current.Effect,
		current.TargetElementIDs,
		current.EffectiveAt,
	)
	require.NoError(t, err)
	profile.Traits[1].EffectRevision = replacement
	checksum, err := profile.CalculateChecksum()

	require.NoError(t, err)
	require.NotEqual(t, original, checksum)
}

func TestRewardProfileRejectsEffectRevisionEffectiveAfterProfile(t *testing.T) {
	at := time.Date(2026, 8, 9, 2, 0, 0, 0, time.UTC)
	profile := rewardProfileFixture(t, at)
	assignment := profile.Traits[0]
	futureEffect, err := NewTraitEffectRevision(
		assignment.EffectRevision.ID,
		assignment.TraitID,
		assignment.EffectRevision.Revision,
		assignment.EffectRevision.Effect,
		assignment.EffectRevision.TargetElementIDs,
		// A microsecond is the smallest gap storage can represent, so a smaller
		// one would be truncated away and prove nothing.
		at.Add(time.Microsecond),
	)
	require.NoError(t, err)
	profile.Traits[0].EffectRevision = futureEffect

	_, err = NewUserRewardProfileRevision(
		profile.ID,
		profile.UserID,
		profile.Revision,
		profile.Source,
		profile.SourceOfferID,
		profile.CommandID,
		profile.RequestChecksum,
		profile.EffectiveAt,
		profile.Traits,
	)
	require.ErrorIs(t, err, ErrInvalidRewardProfile)
}

func rewardProfileFixture(t *testing.T, at time.Time) UserRewardProfileRevision {
	t.Helper()
	traits := make([]RewardProfileTrait, 4)
	for index := range traits {
		kind := TraitTypeTalent
		slot := RewardProfileSlotTalent
		order := index - 1
		if index == 0 {
			kind = TraitTypeRootBone
			slot = RewardProfileSlotRootBone
			order = 0
		}
		effect, err := DecodeTraitEffectSpec([]byte(`{"v":1,"kind":"exp_bonus","scope":"all","flat_bonus":10}`))
		require.NoError(t, err)
		traitID := uuid.New()
		effectRevision, err := NewTraitEffectRevision(uuid.New(), traitID, 1, effect, nil, at)
		require.NoError(t, err)
		traits[index] = RewardProfileTrait{
			TraitID: traitID, TraitType: kind, Slot: slot, SlotOrder: order,
			EffectRevision: effectRevision,
		}
	}
	profile := UserRewardProfileRevision{
		ID: uuid.New(), UserID: uuid.New(), Revision: 1,
		Source: RewardProfileSourceOnboarding, SourceOfferID: uuidPointer(uuid.New()),
		RequestChecksum: [32]byte{1}, EffectiveAt: at, Traits: traits,
	}
	checksum, err := profile.CalculateChecksum()
	require.NoError(t, err)
	profile.AssignmentChecksum = checksum
	return profile
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }
