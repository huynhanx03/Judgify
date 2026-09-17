package http

import (
	"fmt"
	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toRarityID(request *rarityIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse rarity id: %w", err)
	}
	return id, nil
}
func toRarityResponse(result *contract.RarityResponse) *rarityResponse {
	if result == nil {
		return nil
	}
	return &rarityResponse{ID: result.ID.String(), Name: result.Name, Code: result.Code, Weight: result.Weight, Description: result.Description}
}
func toRarityResponses(results []*contract.RarityResponse) []*rarityResponse {
	if results == nil {
		return nil
	}
	values := make([]*rarityResponse, len(results))
	for index, result := range results {
		values[index] = toRarityResponse(result)
	}
	return values
}
func toRarityPage(result *d.Paginated[*contract.RarityResponse]) *d.Paginated[*rarityResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*rarityResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := toRarityResponses(*result.Records)
		page.Records = &values
	}
	return page
}
func toCreateRarityCommand(request *createRarityRequest) *contract.CreateRarityCommand {
	return &contract.CreateRarityCommand{Name: request.Name, Code: request.Code, Weight: request.Weight, Description: request.Description}
}
func toUpdateRarityCommand(request *updateRarityRequest) (uuid.UUID, *contract.UpdateRarityCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parse rarity id: %w", err)
	}
	return id, &contract.UpdateRarityCommand{Name: request.Name, Code: request.Code, Weight: request.Weight, Description: request.Description}, nil
}
