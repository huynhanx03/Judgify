package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"
	commonjson "github.com/huynhanx03/go-common/pkg/encoding/json"
)

const (
	TraitEffectSchemaVersion = 1
	MaximumTraitEffectBytes  = 8 << 10

	MinimumMultiplierDeltaBasisPoints = int64(-10_000)
	MaximumMultiplierDeltaBasisPoints = int64(90_000)
	MinimumTraitExperienceBonus       = int64(-1_000_000_000)
	MaximumTraitExperienceBonus       = int64(1_000_000_000)

	traitEffectChecksumDomain = "judgify.cultivation.trait-effect-revision.v1"
)

type TraitEffectKind string

const (
	TraitEffectExperienceMultiplier TraitEffectKind = "exp_multiplier"
	TraitEffectExperienceBonus      TraitEffectKind = "exp_bonus"
)

type TraitEffectScope string

const (
	TraitEffectScopeAll     TraitEffectScope = "all"
	TraitEffectScopeElement TraitEffectScope = "element"
)

var (
	ErrInvalidTraitEffect = errors.New("cultivation: invalid trait effect")
	traitElementCode      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// TraitEffectSpec is the only production effect language. It deliberately has
// no extension map: a deployment that does not understand a field, kind, or
// version must reject it instead of silently changing reward semantics.
type TraitEffectSpec struct {
	Version                    int              `json:"v"`
	Kind                       TraitEffectKind  `json:"kind"`
	Scope                      TraitEffectScope `json:"scope"`
	ElementCodes               []string         `json:"element_codes,omitempty"`
	MultiplierDeltaBasisPoints *int64           `json:"multiplier_delta_bps,omitempty"`
	FlatBonus                  *int64           `json:"flat_bonus,omitempty"`
}

func DecodeTraitEffectSpec(document []byte) (TraitEffectSpec, error) {
	if len(document) == 0 || len(document) > MaximumTraitEffectBytes {
		return TraitEffectSpec{}, ErrInvalidTraitEffect
	}
	var value TraitEffectSpec
	if err := commonjson.UnmarshalStrictLimit(document, &value, MaximumTraitEffectBytes); err != nil {
		return TraitEffectSpec{}, ErrInvalidTraitEffect
	}
	return NormalizeTraitEffectSpec(value)
}

func NormalizeTraitEffectSpec(value TraitEffectSpec) (TraitEffectSpec, error) {
	normalized := TraitEffectSpec{
		Version:                    value.Version,
		Kind:                       value.Kind,
		Scope:                      value.Scope,
		ElementCodes:               append([]string(nil), value.ElementCodes...),
		MultiplierDeltaBasisPoints: cloneInt64Pointer(value.MultiplierDeltaBasisPoints),
		FlatBonus:                  cloneInt64Pointer(value.FlatBonus),
	}
	if normalized.Version != TraitEffectSchemaVersion ||
		(normalized.Scope != TraitEffectScopeAll && normalized.Scope != TraitEffectScopeElement) {
		return TraitEffectSpec{}, ErrInvalidTraitEffect
	}
	if normalized.Scope == TraitEffectScopeAll {
		if len(normalized.ElementCodes) != 0 {
			return TraitEffectSpec{}, ErrInvalidTraitEffect
		}
	} else if len(normalized.ElementCodes) == 0 || len(normalized.ElementCodes) > 16 {
		return TraitEffectSpec{}, ErrInvalidTraitEffect
	}
	slices.Sort(normalized.ElementCodes)
	for index, code := range normalized.ElementCodes {
		if !traitElementCode.MatchString(code) ||
			(index > 0 && normalized.ElementCodes[index-1] == code) {
			return TraitEffectSpec{}, ErrInvalidTraitEffect
		}
	}
	switch normalized.Kind {
	case TraitEffectExperienceMultiplier:
		if normalized.MultiplierDeltaBasisPoints == nil || normalized.FlatBonus != nil ||
			*normalized.MultiplierDeltaBasisPoints < MinimumMultiplierDeltaBasisPoints ||
			*normalized.MultiplierDeltaBasisPoints > MaximumMultiplierDeltaBasisPoints {
			return TraitEffectSpec{}, ErrInvalidTraitEffect
		}
	case TraitEffectExperienceBonus:
		if normalized.FlatBonus == nil || normalized.MultiplierDeltaBasisPoints != nil ||
			*normalized.FlatBonus < MinimumTraitExperienceBonus ||
			*normalized.FlatBonus > MaximumTraitExperienceBonus {
			return TraitEffectSpec{}, ErrInvalidTraitEffect
		}
	default:
		return TraitEffectSpec{}, ErrInvalidTraitEffect
	}
	return normalized, nil
}

func (value TraitEffectSpec) Valid() bool {
	normalized, err := NormalizeTraitEffectSpec(value)
	return err == nil && value.equal(normalized)
}

func (value TraitEffectSpec) equal(other TraitEffectSpec) bool {
	return value.Version == other.Version && value.Kind == other.Kind &&
		value.Scope == other.Scope && slices.Equal(value.ElementCodes, other.ElementCodes) &&
		equalOptionalInt64(value.MultiplierDeltaBasisPoints, other.MultiplierDeltaBasisPoints) &&
		equalOptionalInt64(value.FlatBonus, other.FlatBonus)
}

// TraitEffectRevision binds a typed effect to the immutable element identities
// resolved from its stable codes and to one database-linearized effective time.
type TraitEffectRevision struct {
	ID               uuid.UUID         `json:"id"`
	TraitID          uuid.UUID         `json:"trait_id"`
	Revision         int64             `json:"revision"`
	Effect           TraitEffectSpec   `json:"effect"`
	TargetElementIDs []uuid.UUID       `json:"target_element_ids,omitempty"`
	Checksum         [sha256.Size]byte `json:"-"`
	EffectiveAt      time.Time         `json:"effective_at"`
}

func NewTraitEffectRevision(
	id uuid.UUID,
	traitID uuid.UUID,
	revision int64,
	effect TraitEffectSpec,
	targetElementIDs []uuid.UUID,
	effectiveAt time.Time,
) (TraitEffectRevision, error) {
	normalized, err := NormalizeTraitEffectSpec(effect)
	if err != nil || id == uuid.Nil || traitID == uuid.Nil || revision < 1 || effectiveAt.IsZero() {
		return TraitEffectRevision{}, ErrInvalidTraitEffect
	}
	// A nil slice hashes as null while storage round-trips it back as an empty
	// array, which would break the checksum for every effect with no element
	// scope. Normalising to a non-nil slice keeps the hashed and stored shapes
	// identical.
	resolved := make([]uuid.UUID, 0, len(targetElementIDs))
	resolved = append(resolved, targetElementIDs...)
	if len(resolved) != len(normalized.ElementCodes) {
		return TraitEffectRevision{}, ErrInvalidTraitEffect
	}
	seen := make(map[uuid.UUID]struct{}, len(resolved))
	for _, elementID := range resolved {
		if elementID == uuid.Nil {
			return TraitEffectRevision{}, ErrInvalidTraitEffect
		}
		if _, duplicate := seen[elementID]; duplicate {
			return TraitEffectRevision{}, ErrInvalidTraitEffect
		}
		seen[elementID] = struct{}{}
	}
	// The checksum covers EffectiveAt, and PostgreSQL timestamptz keeps only
	// microseconds. Truncating here means the value that is hashed is the value
	// that survives a round trip, so a revision written with nanosecond
	// precision still verifies when it is read back.
	value := TraitEffectRevision{
		ID: id, TraitID: traitID, Revision: revision, Effect: normalized,
		TargetElementIDs: resolved,
		EffectiveAt:      effectiveAt.UTC().Truncate(time.Microsecond),
	}
	value.Checksum, err = value.CalculateChecksum()
	if err != nil || !value.Valid() {
		return TraitEffectRevision{}, ErrInvalidTraitEffect
	}
	return value, nil
}

func (value TraitEffectRevision) CalculateChecksum() ([sha256.Size]byte, error) {
	document := struct {
		Domain           string          `json:"domain"`
		ID               uuid.UUID       `json:"id"`
		TraitID          uuid.UUID       `json:"trait_id"`
		Revision         int64           `json:"revision"`
		Effect           TraitEffectSpec `json:"effect"`
		TargetElementIDs []uuid.UUID     `json:"target_element_ids"`
		EffectiveAt      time.Time       `json:"effective_at"`
	}{
		Domain: traitEffectChecksumDomain, ID: value.ID, TraitID: value.TraitID,
		Revision: value.Revision, Effect: value.Effect,
		TargetElementIDs: value.TargetElementIDs, EffectiveAt: value.EffectiveAt.UTC(),
	}
	encoded, err := json.Marshal(document)
	if err != nil || len(encoded) > MaximumTraitEffectBytes {
		return [sha256.Size]byte{}, ErrInvalidTraitEffect
	}
	return sha256.Sum256(encoded), nil
}

func (value TraitEffectRevision) Valid() bool {
	if value.ID == uuid.Nil || value.TraitID == uuid.Nil || value.Revision < 1 ||
		value.EffectiveAt.IsZero() || !value.Effect.Valid() ||
		len(value.TargetElementIDs) != len(value.Effect.ElementCodes) ||
		value.Checksum == ([sha256.Size]byte{}) {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(value.TargetElementIDs))
	for _, elementID := range value.TargetElementIDs {
		if elementID == uuid.Nil {
			return false
		}
		if _, duplicate := seen[elementID]; duplicate {
			return false
		}
		seen[elementID] = struct{}{}
	}
	checksum, err := value.CalculateChecksum()
	return err == nil && subtle.ConstantTimeCompare(checksum[:], value.Checksum[:]) == 1
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func equalOptionalInt64(left, right *int64) bool {
	return (left == nil && right == nil) ||
		(left != nil && right != nil && *left == *right)
}
