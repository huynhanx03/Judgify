package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrTraitConflict = errors.New("cultivation: trait version conflict")

// TraitRarity holds nested rarity info returned from eager loading.
type TraitRarity struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Code   string    `json:"code"`
	Weight int       `json:"weight"`
}

// Trait represents a root bone or talent definition.
type Trait struct {
	ID             uuid.UUID            `json:"id"`
	Code           string               `json:"code"`
	Type           string               `json:"type"`
	Name           string               `json:"name"`
	RarityID       uuid.UUID            `json:"rarity_id"`
	Rarity         *TraitRarity         `json:"rarity"`
	Description    string               `json:"description"`
	DisplayOrder   int                  `json:"display_order"`
	Active         bool                 `json:"active"`
	EffectRevision *TraitEffectRevision `json:"effect_revision,omitempty"`
	Version        int64                `json:"version"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
