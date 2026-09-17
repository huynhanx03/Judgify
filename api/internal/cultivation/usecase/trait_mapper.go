package usecase

import (
	"encoding/hex"

	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
	entity "github.com/huynhanx03/judgify/internal/cultivation/domain"
)

// ToTraitResponse converts Trait entity to response DTO.
func ToTraitResponse(e *entity.Trait) *contract.TraitResponse {
	if e == nil {
		return nil
	}
	resp := &contract.TraitResponse{
		ID:           e.ID,
		Code:         e.Code,
		Type:         e.Type,
		Name:         e.Name,
		Description:  e.Description,
		DisplayOrder: e.DisplayOrder,
		Active:       e.Active,
		Version:      e.Version,
	}
	if e.EffectRevision != nil {
		resp.EffectRevision = &contract.TraitEffectRevisionResponse{
			ID: e.EffectRevision.ID, Revision: e.EffectRevision.Revision,
			Effect:      e.EffectRevision.Effect,
			Checksum:    hex.EncodeToString(e.EffectRevision.Checksum[:]),
			EffectiveAt: e.EffectRevision.EffectiveAt,
		}
	}
	if e.Rarity != nil {
		resp.Rarity = &contract.TraitRarityInfo{
			ID:     e.Rarity.ID,
			Name:   e.Rarity.Name,
			Code:   e.Rarity.Code,
			Weight: e.Rarity.Weight,
		}
	}
	return resp
}

// ToTraitEntityFromCreate converts CreateTraitCommand to entity.
func ToTraitEntityFromCreate(command *contract.CreateTraitCommand) *entity.Trait {
	return &entity.Trait{
		Code: command.Code, Type: command.Type, Name: command.Name, RarityID: command.RarityID,
		Description: command.Description, DisplayOrder: command.DisplayOrder,
		Active: true,
	}
}
