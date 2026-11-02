package contract

import (
	"time"

	"github.com/google/uuid"

	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// CreateTraitCommand contains the data needed to publish a trait.
type CreateTraitCommand struct {
	Code         string
	Type         string
	Name         string
	RarityID     uuid.UUID
	Description  string
	DisplayOrder int
	Effect       entity.TraitEffectSpec
	Reason       string
}

// UpdateTraitCommand contains mutable trait data and the concurrency precondition.
type UpdateTraitCommand struct {
	Type            *string
	Name            *string
	RarityID        *uuid.UUID
	Description     *string
	DisplayOrder    *int
	Active          *bool
	Effect          *entity.TraitEffectSpec
	ExpectedVersion int64
	Reason          string
}

// TraitRarityInfo holds nested rarity data in trait response.
type TraitRarityInfo struct {
	ID     uuid.UUID
	Name   string
	Code   string
	Weight int
}

// TraitResponse represents trait data in API response.
type TraitResponse struct {
	ID             uuid.UUID
	Code           string
	Type           string
	Name           string
	Rarity         *TraitRarityInfo
	Description    string
	DisplayOrder   int
	Active         bool
	EffectRevision *TraitEffectRevisionResponse
	Version        int64
}

type TraitEffectRevisionResponse struct {
	ID          uuid.UUID
	Revision    int64
	Effect      entity.TraitEffectSpec
	Checksum    string
	EffectiveAt time.Time
}
