package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TraitTypeRootBone = "root_bone"
	TraitTypeTalent   = "talent"

	TraitOfferIssued     = "issued"
	TraitOfferConsumed   = "consumed"
	TraitOfferExpired    = "expired"
	TraitOfferSuperseded = "superseded"

	traitOfferChecksumDomain = "judgify.cultivation.trait-offer.v1"
)

var (
	ErrTraitOfferInvalid     = errors.New("cultivation: invalid trait offer")
	ErrTraitOfferUnavailable = errors.New("cultivation: trait offer unavailable")
	ErrTraitOfferExpired     = errors.New("cultivation: trait offer expired")
	ErrTraitOfferSuperseded  = errors.New("cultivation: trait offer superseded")
	ErrTraitOfferConsumed    = errors.New("cultivation: trait offer already consumed")
	ErrTraitOfferQuota       = errors.New("cultivation: trait offer quota exhausted")
	ErrTraitOfferConflict    = errors.New("cultivation: trait offer conflict")
)

type TraitOffer struct {
	ID                uuid.UUID
	PreSessionID      uuid.UUID
	IdempotencyKey    uuid.UUID
	Sequence          int64
	CatalogRevisionID uuid.UUID
	CatalogChecksum   [sha256.Size]byte
	AlgorithmVersion  string
	Root              CatalogTrait
	Talents           []CatalogTrait
	OfferChecksum     [sha256.Size]byte
	Status            string
	Remaining         int
	IssuedAt          time.Time
	ExpiresAt         time.Time
	ConsumedByUserID  *uuid.UUID
	ConsumedAt        *time.Time
}

func (offer TraitOffer) Valid() bool {
	if offer.ID == uuid.Nil || offer.PreSessionID == uuid.Nil ||
		offer.IdempotencyKey == uuid.Nil || offer.Sequence < 1 ||
		offer.CatalogRevisionID == uuid.Nil ||
		offer.CatalogChecksum == ([sha256.Size]byte{}) ||
		offer.AlgorithmVersion != OfferAlgorithmVersion ||
		offer.Root.ID == uuid.Nil || offer.Root.Type != TraitTypeRootBone ||
		!validOfferedCatalogTrait(offer.Root) ||
		len(offer.Talents) != 6 || offer.OfferChecksum == ([sha256.Size]byte{}) ||
		offer.IssuedAt.IsZero() || !offer.ExpiresAt.After(offer.IssuedAt) ||
		offer.Remaining < 0 {
		return false
	}
	seen := map[uuid.UUID]struct{}{offer.Root.ID: {}}
	for _, talent := range offer.Talents {
		if talent.ID == uuid.Nil || talent.Type != TraitTypeTalent ||
			!validOfferedCatalogTrait(talent) {
			return false
		}
		if _, duplicate := seen[talent.ID]; duplicate {
			return false
		}
		seen[talent.ID] = struct{}{}
	}
	switch offer.Status {
	case TraitOfferConsumed:
		return offer.ConsumedByUserID != nil && *offer.ConsumedByUserID != uuid.Nil &&
			offer.ConsumedAt != nil && !offer.ConsumedAt.IsZero()
	case TraitOfferIssued, TraitOfferExpired, TraitOfferSuperseded:
		return offer.ConsumedByUserID == nil && offer.ConsumedAt == nil
	default:
		return false
	}
}

func validOfferedCatalogTrait(value CatalogTrait) bool {
	return value.ID != uuid.Nil && value.Revision >= 1 && value.Code != "" &&
		value.Name != "" && value.RarityID != uuid.Nil && value.RarityCode != "" &&
		value.RarityName != "" && value.Weight > 0 && value.DisplayOrder >= 0 &&
		value.EffectRevision.Valid() && value.EffectRevision.TraitID == value.ID &&
		value.EffectRevision.Revision == value.Revision
}

// TraitOfferChecksum returns the canonical digest that binds an offer to the
// exact pre-session, catalog revision, ordered trait revisions, and lifetime.
// Keeping this canonicalization in the domain prevents adapters from drifting
// to incompatible evidence formats.
func TraitOfferChecksum(offer TraitOffer) ([sha256.Size]byte, error) {
	type traitIdentity struct {
		ID               uuid.UUID `json:"id"`
		Revision         int64     `json:"revision"`
		EffectRevisionID uuid.UUID `json:"effect_revision_id"`
		EffectChecksum   string    `json:"effect_checksum"`
	}
	talents := make([]traitIdentity, len(offer.Talents))
	for index, talent := range offer.Talents {
		talents[index] = traitIdentity{
			ID: talent.ID, Revision: talent.Revision,
			EffectRevisionID: talent.EffectRevision.ID,
			EffectChecksum:   checksumHex(talent.EffectRevision.Checksum),
		}
	}
	payload, err := json.Marshal(struct {
		Domain            string          `json:"domain"`
		ID                uuid.UUID       `json:"id"`
		PreSessionID      uuid.UUID       `json:"pre_session_id"`
		IdempotencyKey    uuid.UUID       `json:"idempotency_key"`
		Sequence          int64           `json:"sequence"`
		CatalogRevisionID uuid.UUID       `json:"catalog_revision_id"`
		CatalogChecksum   [32]byte        `json:"catalog_checksum"`
		AlgorithmVersion  string          `json:"algorithm_version"`
		Root              traitIdentity   `json:"root"`
		Talents           []traitIdentity `json:"talents"`
		IssuedAt          time.Time       `json:"issued_at"`
		ExpiresAt         time.Time       `json:"expires_at"`
	}{
		Domain: traitOfferChecksumDomain, ID: offer.ID,
		PreSessionID: offer.PreSessionID, IdempotencyKey: offer.IdempotencyKey,
		Sequence: offer.Sequence, CatalogRevisionID: offer.CatalogRevisionID,
		CatalogChecksum: offer.CatalogChecksum, AlgorithmVersion: offer.AlgorithmVersion,
		Root: traitIdentity{
			ID: offer.Root.ID, Revision: offer.Root.Revision,
			EffectRevisionID: offer.Root.EffectRevision.ID,
			EffectChecksum:   checksumHex(offer.Root.EffectRevision.Checksum),
		},
		Talents: talents, IssuedAt: offer.IssuedAt.UTC(), ExpiresAt: offer.ExpiresAt.UTC(),
	})
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}
