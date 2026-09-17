package http

import (
	"fmt"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toLevelID(request *levelIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse level id: %w", err)
	}
	return id, nil
}

func toLevelResponse(result *contract.LevelResponse) *levelResponse {
	if result == nil {
		return nil
	}
	return &levelResponse{ID: result.ID.String(), Name: result.Name, MinExp: result.MinExp, Description: result.Description}
}

func toLevelResponses(results []*contract.LevelResponse) []*levelResponse {
	if results == nil {
		return nil
	}
	values := make([]*levelResponse, len(results))
	for index, result := range results {
		values[index] = toLevelResponse(result)
	}
	return values
}

func toLevelPage(result *d.Paginated[*contract.LevelResponse]) *d.Paginated[*levelResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*levelResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := toLevelResponses(*result.Records)
		page.Records = &values
	}
	return page
}

func toCreateLevelCommand(request *createLevelRequest) *contract.CreateLevelCommand {
	return &contract.CreateLevelCommand{Name: request.Name, MinExp: request.MinExp, Description: request.Description}
}

func toUpdateLevelCommand(request *updateLevelRequest) (uuid.UUID, *contract.UpdateLevelCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parse level id: %w", err)
	}
	return id, &contract.UpdateLevelCommand{Name: request.Name, MinExp: request.MinExp, Description: request.Description}, nil
}
