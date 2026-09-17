package http

import (
	"fmt"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toCreateTraitCommand(request *createTraitRequest) (*contract.CreateTraitCommand, error) {
	rarityID, err := uuid.Parse(request.RarityID)
	if err != nil {
		return nil, fmt.Errorf("parse trait rarity id: %w", err)
	}
	return &contract.CreateTraitCommand{
		Code: request.Code, Type: request.Type, Name: request.Name, RarityID: rarityID,
		Description: request.Description, DisplayOrder: request.DisplayOrder, Effect: request.Effect, Reason: request.Reason,
	}, nil
}

func toTraitID(request *traitIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse trait id: %w", err)
	}
	return id, nil
}
func toUpdateTraitCommand(request *updateTraitRequest) (uuid.UUID, *contract.UpdateTraitCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parse trait id: %w", err)
	}
	var rarityID *uuid.UUID
	if request.RarityID != nil {
		parsed, err := uuid.Parse(*request.RarityID)
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("parse trait rarity id: %w", err)
		}
		rarityID = &parsed
	}
	return id, &contract.UpdateTraitCommand{Type: request.Type, Name: request.Name, RarityID: rarityID, Description: request.Description, DisplayOrder: request.DisplayOrder, Active: request.Active, Effect: request.Effect, ExpectedVersion: request.ExpectedVersion, Reason: request.Reason}, nil
}

func toTraitResponse(result *contract.TraitResponse) *traitResponse {
	if result == nil {
		return nil
	}
	response := &traitResponse{ID: result.ID.String(), Code: result.Code, Type: result.Type, Name: result.Name, Description: result.Description, DisplayOrder: result.DisplayOrder, Active: result.Active, Version: result.Version}
	if result.Rarity != nil {
		response.Rarity = &traitRarityResponse{ID: result.Rarity.ID.String(), Name: result.Rarity.Name, Code: result.Rarity.Code, Weight: result.Rarity.Weight}
	}
	if result.EffectRevision != nil {
		response.EffectRevision = &traitEffectRevisionResponse{ID: result.EffectRevision.ID.String(), Revision: result.EffectRevision.Revision, Effect: result.EffectRevision.Effect, Checksum: result.EffectRevision.Checksum, EffectiveAt: result.EffectRevision.EffectiveAt}
	}
	return response
}
func toTraitResponses(results []*contract.TraitResponse) []*traitResponse {
	if results == nil {
		return nil
	}
	values := make([]*traitResponse, len(results))
	for index, result := range results {
		values[index] = toTraitResponse(result)
	}
	return values
}
func toTraitPage(result *d.Paginated[*contract.TraitResponse]) *d.Paginated[*traitResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*traitResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := toTraitResponses(*result.Records)
		page.Records = &values
	}
	return page
}
