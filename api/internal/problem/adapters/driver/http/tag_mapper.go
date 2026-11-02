package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func toCreateTagCommand(request *CreateTagRequest) (contract.CreateTagCommand, error) {
	elementIDs, err := parseTagElementIDs(request.ElementIDs)
	if err != nil {
		return contract.CreateTagCommand{}, err
	}
	return contract.CreateTagCommand{Name: request.Name, ElementIDs: elementIDs}, nil
}

func toUpdateTagCommand(request *UpdateTagRequest) (uuid.UUID, contract.UpdateTagCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, contract.UpdateTagCommand{}, fmt.Errorf("parse tag id: %w", err)
	}

	command := contract.UpdateTagCommand{Name: request.Name}
	if request.ElementIDs != nil {
		elementIDs, err := parseTagElementIDs(*request.ElementIDs)
		if err != nil {
			return uuid.Nil, contract.UpdateTagCommand{}, err
		}
		command.ElementIDs = &elementIDs
	}
	return id, command, nil
}

func toTagID(request *tagIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse tag id: %w", err)
	}
	return id, nil
}

func parseTagElementIDs(rawIDs []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(rawIDs))
	for index, rawID := range rawIDs {
		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, fmt.Errorf("parse tag element id at index %d: %w", index, err)
		}
		ids[index] = id
	}
	return ids, nil
}

func toTagResponse(result *contract.TagResult) *TagResponse {
	if result == nil {
		return nil
	}
	elements := make([]*TagElementResponse, len(result.Elements))
	for index, element := range result.Elements {
		elements[index] = &TagElementResponse{ID: element.ID.String(), Name: element.Name, Code: element.Code}
	}
	return &TagResponse{ID: result.ID.String(), Name: result.Name, Elements: elements, Version: result.Version}
}
