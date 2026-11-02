package domain

import (
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	OfferAlgorithmVersion = "weighted-without-replacement-v1"
	catalogChecksumDomain = "judgify.cultivation.catalog.v1"
)

// CatalogTrait is the immutable, display-safe weighted trait snapshot used by
// offer generation. Metadata/effects stay out of the anonymous response.
type CatalogTrait struct {
	ID             uuid.UUID           `json:"id"`
	Revision       int64               `json:"revision"`
	Code           string              `json:"code"`
	Type           string              `json:"type"`
	Name           string              `json:"name"`
	Description    string              `json:"description,omitempty"`
	DisplayOrder   int                 `json:"display_order"`
	RarityID       uuid.UUID           `json:"rarity_id"`
	RarityCode     string              `json:"rarity_code"`
	RarityName     string              `json:"rarity_name"`
	Weight         int                 `json:"weight"`
	EffectRevision TraitEffectRevision `json:"effect_revision"`
}

type CatalogRevision struct {
	ID               uuid.UUID
	Revision         int64
	AlgorithmVersion string
	Checksum         [sha256.Size]byte
	RootPool         []CatalogTrait
	TalentPool       []CatalogTrait
	MaximumOffers    int
	OfferTTL         time.Duration
	CreatedAt        time.Time
}

func (catalog CatalogRevision) Valid() bool {
	if catalog.ID == uuid.Nil || catalog.Revision < 1 ||
		catalog.AlgorithmVersion != OfferAlgorithmVersion ||
		catalog.Checksum == ([sha256.Size]byte{}) || catalog.MaximumOffers < 1 ||
		catalog.MaximumOffers > 10 || catalog.OfferTTL < time.Minute ||
		catalog.OfferTTL > 24*time.Hour || catalog.CreatedAt.IsZero() ||
		len(catalog.RootPool) < 1 || len(catalog.TalentPool) < 6 {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(catalog.RootPool)+len(catalog.TalentPool))
	for index, candidate := range append(
		append([]CatalogTrait(nil), catalog.RootPool...), catalog.TalentPool...,
	) {
		if candidate.ID == uuid.Nil || candidate.Revision < 1 || candidate.Weight < 1 ||
			candidate.Code == "" || candidate.Name == "" || candidate.RarityID == uuid.Nil ||
			candidate.RarityCode == "" || candidate.RarityName == "" ||
			candidate.DisplayOrder < 0 || !candidate.EffectRevision.Valid() ||
			candidate.EffectRevision.TraitID != candidate.ID ||
			candidate.EffectRevision.Revision != candidate.Revision ||
			(index < len(catalog.RootPool) && candidate.Type != TraitTypeRootBone) ||
			(index >= len(catalog.RootPool) && candidate.Type != TraitTypeTalent) {
			return false
		}
		if _, duplicate := seen[candidate.ID]; duplicate {
			return false
		}
		seen[candidate.ID] = struct{}{}
	}
	return true
}

func CatalogChecksum(
	roots []CatalogTrait,
	talents []CatalogTrait,
	maximumOffers int,
	offerTTL time.Duration,
) ([sha256.Size]byte, error) {
	payload, err := json.Marshal(struct {
		Domain           string         `json:"domain"`
		AlgorithmVersion string         `json:"algorithm_version"`
		MaximumOffers    int            `json:"maximum_offers"`
		OfferTTLSeconds  int64          `json:"offer_ttl_seconds"`
		RootPool         []CatalogTrait `json:"root_pool"`
		TalentPool       []CatalogTrait `json:"talent_pool"`
	}{
		Domain: catalogChecksumDomain, AlgorithmVersion: OfferAlgorithmVersion,
		MaximumOffers: maximumOffers, OfferTTLSeconds: int64(offerTTL / time.Second),
		RootPool: roots, TalentPool: talents,
	})
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(payload), nil
}
