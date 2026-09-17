package contract

import (
	"time"

	"github.com/google/uuid"
)

type CreateTraitOfferRequest struct {
	IdempotencyKey uuid.UUID `json:"-"`
	PreSessionID   uuid.UUID `json:"-"`
}

type TraitOfferTraitResponse struct {
	ID          uuid.UUID `json:"id"`
	Revision    int64     `json:"revision"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	RarityID    uuid.UUID `json:"rarity_id"`
	RarityCode  string    `json:"rarity_code"`
	RarityName  string    `json:"rarity_name"`
}

type TraitOfferResponse struct {
	ID                uuid.UUID                  `json:"id"`
	Sequence          int64                      `json:"sequence"`
	CatalogRevisionID uuid.UUID                  `json:"catalog_revision_id"`
	CatalogChecksum   string                     `json:"catalog_checksum"`
	AlgorithmVersion  string                     `json:"algorithm_version"`
	OfferChecksum     string                     `json:"offer_checksum"`
	RootBone          *TraitOfferTraitResponse   `json:"root_bone"`
	Talents           []*TraitOfferTraitResponse `json:"talents"`
	RemainingOffers   int                        `json:"remaining_offers"`
	ExpiresAt         time.Time                  `json:"expires_at"`
	ServerTime        time.Time                  `json:"server_time"`
}
