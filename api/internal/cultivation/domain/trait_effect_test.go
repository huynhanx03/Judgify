package domain

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTraitEffectSpecUsesClosedFixedPointContract(t *testing.T) {
	value, err := DecodeTraitEffectSpec([]byte(`{
		"v":1,
		"kind":"exp_multiplier",
		"scope":"element",
		"element_codes":["fire","water"],
		"multiplier_delta_bps":2500
	}`))

	require.NoError(t, err)
	require.Equal(t, int64(2_500), *value.MultiplierDeltaBasisPoints)
	require.Equal(t, []string{"fire", "water"}, value.ElementCodes)
	require.Nil(t, value.FlatBonus)
}

func TestTraitEffectSpecRejectsOpenOrFloatingPointDocuments(t *testing.T) {
	tests := []string{
		`{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":1.5}`,
		`{"v":1,"kind":"exp_multiplier","scope":"all","multiplier_delta_bps":2500,"daily_limit":3}`,
		`{"v":2,"kind":"exp_bonus","scope":"all","flat_bonus":10}`,
		`{"v":1,"kind":"unknown","scope":"all","flat_bonus":10}`,
		`{"v":1,"kind":"exp_bonus","scope":"element","element_codes":["fire","fire"],"flat_bonus":10}`,
		`{"v":1,"kind":"exp_bonus","scope":"element","element_codes":["unknown element"],"flat_bonus":10}`,
		`{"v":1,"kind":"exp_bonus","scope":"all","element_codes":["fire"],"flat_bonus":10}`,
		`{"v":1,"kind":"exp_bonus","scope":"all","flat_bonus":1000000001}`,
	}
	for _, document := range tests {
		_, err := DecodeTraitEffectSpec([]byte(document))
		require.ErrorIs(t, err, ErrInvalidTraitEffect, document)
	}
}

func TestTraitEffectRevisionChecksumBindsResolvedElementsAndEffectiveTime(t *testing.T) {
	effect, err := DecodeTraitEffectSpec([]byte(`{
		"v":1,"kind":"exp_bonus","scope":"element",
		"element_codes":["fire"],"flat_bonus":25
	}`))
	require.NoError(t, err)
	resolved := []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000101")}
	revision, err := NewTraitEffectRevision(
		uuid.MustParse("00000000-0000-4000-8000-000000000102"),
		uuid.MustParse("00000000-0000-4000-8000-000000000103"),
		7,
		effect,
		resolved,
		time.Date(2026, 8, 9, 1, 2, 3, 0, time.UTC),
	)
	require.NoError(t, err)
	require.True(t, revision.Valid())

	tampered := revision
	tampered.TargetElementIDs[0] = uuid.New()
	require.False(t, tampered.Valid())

	encoded, err := json.Marshal(revision.Effect)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), ".")
}

// PostgreSQL timestamptz keeps microseconds, so a revision hashed at nanosecond
// precision would stop verifying the moment it was read back from storage.
func TestTraitEffectRevisionSurvivesMicrosecondStorageRounding(t *testing.T) {
	effect, err := DecodeTraitEffectSpec([]byte(`{
		"v":1,"kind":"exp_bonus","scope":"all","flat_bonus":10
	}`))
	require.NoError(t, err)
	nanosecond := time.Date(2026, 8, 9, 1, 2, 3, 123456789, time.UTC)
	revision, err := NewTraitEffectRevision(
		uuid.MustParse("00000000-0000-4000-8000-000000000201"),
		uuid.MustParse("00000000-0000-4000-8000-000000000202"),
		1,
		effect,
		nil,
		nanosecond,
	)
	require.NoError(t, err)
	require.Equal(t, revision.EffectiveAt, revision.EffectiveAt.Truncate(time.Microsecond))

	stored := revision
	stored.EffectiveAt = revision.EffectiveAt.Truncate(time.Microsecond)
	require.True(t, stored.Valid(), "revision must still verify after storage rounding")
}

// An effect with no element scope is persisted as an empty JSON array and read
// back as an empty slice. Hashing a nil slice instead would make every such
// revision fail verification the first time it is loaded.
func TestTraitEffectRevisionWithoutElementsSurvivesStorageRoundTrip(t *testing.T) {
	effect, err := DecodeTraitEffectSpec([]byte(`{
		"v":1,"kind":"exp_bonus","scope":"all","flat_bonus":10
	}`))
	require.NoError(t, err)
	revision, err := NewTraitEffectRevision(
		uuid.MustParse("00000000-0000-4000-8000-000000000301"),
		uuid.MustParse("00000000-0000-4000-8000-000000000302"),
		1,
		effect,
		nil,
		time.Date(2026, 8, 9, 1, 2, 3, 0, time.UTC),
	)
	require.NoError(t, err)

	encoded, err := json.Marshal(revision.TargetElementIDs)
	require.NoError(t, err)
	require.Equal(t, "[]", string(encoded), "an empty scope must persist as a JSON array")

	var decoded []uuid.UUID
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	stored := revision
	stored.TargetElementIDs = decoded
	require.True(t, stored.Valid(), "revision must still verify after a storage round trip")
}

// The persisted catalog stores its pools as JSON, and a revision's checksum is
// excluded from that JSON. A reader must therefore derive the checksum again;
// this pins the property the reader depends on.
func TestCatalogTraitJSONOmitsEffectChecksumSoReadersMustDeriveIt(t *testing.T) {
	effect, err := DecodeTraitEffectSpec([]byte(`{
		"v":1,"kind":"exp_bonus","scope":"element",
		"element_codes":["fire"],"flat_bonus":10
	}`))
	require.NoError(t, err)
	revision, err := NewTraitEffectRevision(
		uuid.MustParse("00000000-0000-4000-8000-000000000401"),
		uuid.MustParse("00000000-0000-4000-8000-000000000402"),
		1,
		effect,
		[]uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000403")},
		time.Date(2026, 8, 9, 1, 2, 3, 0, time.UTC),
	)
	require.NoError(t, err)
	require.True(t, revision.Valid())

	encoded, err := json.Marshal(revision)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "checksum")

	var restored TraitEffectRevision
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.Equal(t, [sha256.Size]byte{}, restored.Checksum)
	require.False(t, restored.Valid(), "a restored revision has no checksum until one is derived")

	restored.Checksum, err = restored.CalculateChecksum()
	require.NoError(t, err)
	require.True(t, restored.Valid(), "deriving the checksum must restore validity")
	require.Equal(t, revision.Checksum, restored.Checksum)
}
