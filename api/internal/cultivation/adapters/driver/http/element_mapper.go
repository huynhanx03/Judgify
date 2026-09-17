package http

import (
	"fmt"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toElementID(request *elementIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse element id: %w", err)
	}
	return id, nil
}

func toElementResponse(result *contract.ElementResponse) *elementResponse {
	if result == nil {
		return nil
	}
	return &elementResponse{ID: result.ID.String(), Name: result.Name, Code: result.Code, Description: result.Description, Version: result.Version}
}

func toElementResponses(results []*contract.ElementResponse) []*elementResponse {
	if results == nil {
		return nil
	}
	values := make([]*elementResponse, len(results))
	for index, result := range results {
		values[index] = toElementResponse(result)
	}
	return values
}

func toElementPage(result *d.Paginated[*contract.ElementResponse]) *d.Paginated[*elementResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*elementResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := toElementResponses(*result.Records)
		page.Records = &values
	}
	return page
}

func toCreateElementCommand(request *createElementRequest) *contract.CreateElementCommand {
	return &contract.CreateElementCommand{Name: request.Name, Code: request.Code, Description: request.Description}
}

func toUpdateElementCommand(request *updateElementRequest) (uuid.UUID, *contract.UpdateElementCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parse element id: %w", err)
	}
	return id, &contract.UpdateElementCommand{Name: request.Name, Code: request.Code, Description: request.Description}, nil
}
