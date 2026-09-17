package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCalculateSolveRewardGoldenFixedPointVectors(t *testing.T) {
	fire := uuid.MustParse("00000000-0000-4000-8000-000000000201")
	water := uuid.MustParse("00000000-0000-4000-8000-000000000202")
	profile := rewardCalculationProfile(t, fire)
	rule := DefaultRewardRuleRevision()
	rule.Configuration.FlatBonus = 3
	rule.Checksum, _ = rule.CalculateChecksum()

	reward, err := CalculateSolveReward(101, rule, profile, []RewardElementInput{
		{ElementID: fire, WeightBasisPoints: 5_000},
		{ElementID: water, WeightBasisPoints: 5_000},
	})

	require.NoError(t, err)
	// Rule 10,000 + all +2,500 + matched fire +5,000; 101*1.75 rounds to 177,
	// then rule/all/matched bonuses 3+5+2.
	require.Equal(t, int64(187), reward.TotalExperience)
	require.Equal(t, []RewardElementResult{
		{ElementID: fire, Experience: 98},
		{ElementID: water, Experience: 71},
	}, reward.Elements)
}

func TestCalculateSolveRewardRejectsAggregateOverflowAndInvalidHistory(t *testing.T) {
	profile := rewardCalculationProfile(t, uuid.New())
	rule := DefaultRewardRuleRevision()

	_, err := CalculateSolveReward(^int64(0)>>1, rule, profile, nil)
	require.ErrorIs(t, err, ErrRewardCalculation)

	profile.AssignmentChecksum[0] ^= 0xff
	_, err = CalculateSolveReward(100, rule, profile, nil)
	require.ErrorIs(t, err, ErrInvalidRewardProfile)
}

func rewardCalculationProfile(t *testing.T, fire uuid.UUID) UserRewardProfileRevision {
	t.Helper()
	at := time.Date(2026, 8, 9, 3, 0, 0, 0, time.UTC)
	specifications := []struct {
		document string
		targets  []uuid.UUID
	}{
		{`{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":2500}`, nil},
		{`{"v":1,"kind":"exp_multiplier","scope":"element","element_codes":["fire"],"multiplier_delta_bps":5000}`, []uuid.UUID{fire}},
		{`{"v":1,"kind":"exp_bonus","scope":"all","flat_bonus":5}`, nil},
		{`{"v":1,"kind":"exp_bonus","scope":"element","element_codes":["fire"],"flat_bonus":2}`, []uuid.UUID{fire}},
	}
	traits := make([]RewardProfileTrait, len(specifications))
	for index, specification := range specifications {
		effect, err := DecodeTraitEffectSpec([]byte(specification.document))
		require.NoError(t, err)
		traitID := uuid.New()
		revision, err := NewTraitEffectRevision(uuid.New(), traitID, 1, effect, specification.targets, at)
		require.NoError(t, err)
		traits[index] = RewardProfileTrait{
			TraitID: traitID, TraitType: TraitTypeTalent,
			Slot: RewardProfileSlotTalent, SlotOrder: index - 1,
			EffectRevision: revision,
		}
	}
	traits[0].TraitType = TraitTypeRootBone
	traits[0].Slot = RewardProfileSlotRootBone
	traits[0].SlotOrder = 0
	profile := UserRewardProfileRevision{
		ID: uuid.New(), UserID: uuid.New(), Revision: 1,
		Source: RewardProfileSourceAdministrative, CommandID: uuidPointer(uuid.New()),
		RequestChecksum: [32]byte{1}, EffectiveAt: at, Traits: traits,
	}
	profile.AssignmentChecksum, _ = profile.CalculateChecksum()
	return profile
}
