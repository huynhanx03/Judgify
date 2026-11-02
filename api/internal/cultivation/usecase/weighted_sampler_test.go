package usecase

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

func TestSampleWeightedWithoutReplacementNeverReturnsDuplicateTraits(t *testing.T) {
	t.Parallel()

	candidates := make([]entity.CatalogTrait, 8)
	for index := range candidates {
		traitID := uuid.MustParse("00000000-0000-0000-0000-00000000000" + string(rune('1'+index)))
		candidates[index] = validWeightedSamplerCandidate(t, traitID, index+1)
	}
	selected, err := sampleWeightedWithoutReplacement(
		bytes.NewReader(make([]byte, 64)),
		candidates,
		6,
	)
	require.NoError(t, err)
	require.Len(t, selected, 6)
	seen := make(map[uuid.UUID]struct{}, len(selected))
	for _, candidate := range selected {
		_, duplicate := seen[candidate.ID]
		require.False(t, duplicate)
		seen[candidate.ID] = struct{}{}
	}
}

func validWeightedSamplerCandidate(t *testing.T, traitID uuid.UUID, weight int) entity.CatalogTrait {
	t.Helper()
	bonus := int64(0)
	effect, err := entity.NewTraitEffectRevision(
		uuid.New(), traitID, 1,
		entity.TraitEffectSpec{
			Version:   entity.TraitEffectSchemaVersion,
			Kind:      entity.TraitEffectExperienceBonus,
			Scope:     entity.TraitEffectScopeAll,
			FlatBonus: &bonus,
		},
		nil,
		time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	return entity.CatalogTrait{
		ID:             traitID,
		Revision:       1,
		Code:           "candidate",
		Type:           entity.TraitTypeTalent,
		Name:           "Candidate",
		RarityID:       uuid.New(),
		RarityCode:     "common",
		RarityName:     "Common",
		Weight:         weight,
		EffectRevision: effect,
	}
}

func TestSampleWeightedWithoutReplacementRejectsInvalidPoolAndEntropyFailure(t *testing.T) {
	t.Parallel()

	candidates := []entity.CatalogTrait{validWeightedSamplerCandidate(
		t,
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		2,
	)}
	_, err := sampleWeightedWithoutReplacement(nil, candidates, 1)
	require.ErrorIs(t, err, entity.ErrTraitOfferInvalid)
	_, err = sampleWeightedWithoutReplacement(bytes.NewReader(nil), candidates, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, errTraitOfferEntropy))
}
