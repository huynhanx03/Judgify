package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func toCreateMaterialCommand(request *createMaterialRequest) (*contract.CreateMaterialCommand, error) {
	commandID, err := uuid.Parse(request.CommandID)
	if err != nil {
		return nil, fmt.Errorf("parse material command id: %w", err)
	}
	difficultyID, err := uuid.Parse(request.DifficultyID)
	if err != nil {
		return nil, fmt.Errorf("parse material difficulty id: %w", err)
	}
	categoryID, err := uuid.Parse(request.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("parse material category id: %w", err)
	}
	tagIDs, err := parseMaterialTagIDs(request.TagIDs)
	if err != nil {
		return nil, err
	}
	return &contract.CreateMaterialCommand{
		CommandID: commandID, Slug: request.Slug, Title: request.Title, Description: request.Description,
		Content: request.Content, DifficultyID: difficultyID, CategoryID: categoryID, TagIDs: tagIDs,
		ChangeSummary: request.ChangeSummary, Status: request.Status, Visibility: request.Visibility,
	}, nil
}

func toUpdateMaterialCommand(request *updateMaterialRequest) (uuid.UUID, *contract.UpdateMaterialCommand, error) {
	materialID, commandID, err := parseMaterialLifecycleIDs(request.ID, request.CommandID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	difficultyID, err := parseOptionalMaterialID(request.DifficultyID, "difficulty")
	if err != nil {
		return uuid.Nil, nil, err
	}
	categoryID, err := parseOptionalMaterialID(request.CategoryID, "category")
	if err != nil {
		return uuid.Nil, nil, err
	}
	tagIDs, err := parseOptionalMaterialTagIDs(request.TagIDs)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return materialID, &contract.UpdateMaterialCommand{
		CommandID: commandID, Title: request.Title, Description: request.Description, Content: request.Content,
		DifficultyID: difficultyID, CategoryID: categoryID, TagIDs: tagIDs,
		ExpectedVersion: request.ExpectedVersion, ChangeSummary: request.ChangeSummary,
		Status: request.Status, Visibility: request.Visibility,
	}, nil
}

func parseOptionalMaterialID(raw *string, name string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, fmt.Errorf("parse material %s id: %w", name, err)
	}
	return &id, nil
}

func parseMaterialTagIDs(raw []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(raw))
	for index, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("parse material tag id at index %d: %w", index, err)
		}
		ids[index] = id
	}
	return ids, nil
}

func parseOptionalMaterialTagIDs(raw *[]string) (*[]uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	ids, err := parseMaterialTagIDs(*raw)
	if err != nil {
		return nil, err
	}
	return &ids, nil
}

func toMaterialSearchQuery(request *materialSearchRequest, authenticated bool) (ports.SearchQuery, error) {
	categoryID, err := parseOptionalMaterialID(request.CategoryID, "category")
	if err != nil {
		return ports.SearchQuery{}, err
	}
	difficultyID, err := parseOptionalMaterialID(request.DifficultyID, "difficulty")
	if err != nil {
		return ports.SearchQuery{}, err
	}
	tagIDs, err := parseMaterialTagIDs(request.TagIDs)
	if err != nil {
		return ports.SearchQuery{}, err
	}
	return ports.SearchQuery{
		Text: request.Text, CategoryID: categoryID, TagIDs: tagIDs, DifficultyID: difficultyID,
		Sort: request.Sort, Cursor: request.Cursor, Limit: request.Limit, Authenticated: authenticated,
	}, nil
}

func toAdminMaterialSearchQuery(request *adminMaterialSearchRequest) (ports.SearchQuery, error) {
	categoryID, err := parseOptionalMaterialID(request.CategoryID, "category")
	if err != nil {
		return ports.SearchQuery{}, err
	}
	difficultyID, err := parseOptionalMaterialID(request.DifficultyID, "difficulty")
	if err != nil {
		return ports.SearchQuery{}, err
	}
	tagIDs, err := parseMaterialTagIDs(request.TagIDs)
	if err != nil {
		return ports.SearchQuery{}, err
	}
	var status *entity.MaterialState
	if request.Status != nil {
		value := entity.MaterialState(*request.Status)
		status = &value
	}
	return ports.SearchQuery{
		Text: request.Text, CategoryID: categoryID, TagIDs: tagIDs, DifficultyID: difficultyID,
		Status: status, Sort: "updated", Cursor: request.Cursor, Limit: request.Limit, Authenticated: true,
	}, nil
}
