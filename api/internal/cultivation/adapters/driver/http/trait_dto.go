package http

import (
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
	"time"
)

type createTraitRequest struct {
	Code         string                 `json:"code" validate:"required,min=1,max=64"`
	Type         string                 `json:"type" validate:"required,oneof=root_bone talent"`
	Name         string                 `json:"name" validate:"required,min=1,max=100"`
	RarityID     string                 `json:"rarity_id" validate:"required,uuid"`
	Description  string                 `json:"description" validate:"omitempty,max=500"`
	DisplayOrder int                    `json:"display_order" validate:"min=0,max=1000000"`
	Effect       entity.TraitEffectSpec `json:"effect" validate:"required"`
	Reason       string                 `json:"reason" validate:"required,min=3,max=1024"`
}

type updateTraitRequest struct {
	ID              string                  `uri:"id" validate:"required,uuid"`
	Type            *string                 `json:"type" validate:"omitempty,oneof=root_bone talent"`
	Name            *string                 `json:"name" validate:"omitempty,min=1,max=100"`
	RarityID        *string                 `json:"rarity_id" validate:"omitempty,uuid"`
	Description     *string                 `json:"description" validate:"omitempty,max=500"`
	DisplayOrder    *int                    `json:"display_order" validate:"omitempty,min=0,max=1000000"`
	Active          *bool                   `json:"active"`
	Effect          *entity.TraitEffectSpec `json:"effect"`
	ExpectedVersion int64                   `json:"expected_version" validate:"min=1"`
	Reason          string                  `json:"reason" validate:"required,min=3,max=1024"`
}
type traitIDRequest struct {
	ID string `uri:"id" validate:"required,uuid"`
}
type deleteTraitRequest struct {
	ID              string `uri:"id" validate:"required,uuid"`
	ExpectedVersion int64  `json:"expected_version" validate:"min=1"`
	Reason          string `json:"reason" validate:"required,min=3,max=1024"`
}
type traitRarityResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Weight int    `json:"weight"`
}
type traitEffectRevisionResponse struct {
	ID          string                 `json:"id"`
	Revision    int64                  `json:"revision"`
	Effect      entity.TraitEffectSpec `json:"effect"`
	Checksum    string                 `json:"checksum"`
	EffectiveAt time.Time              `json:"effective_at"`
}
type traitResponse struct {
	ID             string                       `json:"id"`
	Code           string                       `json:"code"`
	Type           string                       `json:"type"`
	Name           string                       `json:"name"`
	Rarity         *traitRarityResponse         `json:"rarity"`
	Description    string                       `json:"description,omitempty"`
	DisplayOrder   int                          `json:"display_order"`
	Active         bool                         `json:"active"`
	EffectRevision *traitEffectRevisionResponse `json:"effect_revision,omitempty"`
	Version        int64                        `json:"version"`
}
