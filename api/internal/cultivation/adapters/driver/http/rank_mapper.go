package http

import (
	"fmt"

	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"
	contract "github.com/huynhanx03/judgify/internal/cultivation/contract"
)

func toRankID(request *rankIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse rank id: %w", err)
	}
	return id, nil
}

func toRankResponse(result *contract.RankResponse) *rankResponse {
	if result == nil {
		return nil
	}
	return &rankResponse{ID: result.ID.String(), Name: result.Name, MinRating: result.MinRating, Description: result.Description}
}

func toRankResponses(results []*contract.RankResponse) []*rankResponse {
	if results == nil {
		return nil
	}
	values := make([]*rankResponse, len(results))
	for index, result := range results {
		values[index] = toRankResponse(result)
	}
	return values
}

func toRankPage(result *d.Paginated[*contract.RankResponse]) *d.Paginated[*rankResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*rankResponse]{Pagination: result.Pagination}
	if result.Records != nil {
		values := toRankResponses(*result.Records)
		page.Records = &values
	}
	return page
}

func toCreateRankCommand(request *createRankRequest) *contract.CreateRankCommand {
	return &contract.CreateRankCommand{Name: request.Name, MinRating: request.MinRating, Description: request.Description}
}

func toUpdateRankCommand(request *updateRankRequest) (uuid.UUID, *contract.UpdateRankCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("parse rank id: %w", err)
	}
	return id, &contract.UpdateRankCommand{Name: request.Name, MinRating: request.MinRating, Description: request.Description}, nil
}
