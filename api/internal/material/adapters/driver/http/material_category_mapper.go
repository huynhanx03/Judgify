package http

import (
	"fmt"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
)

func toMaterialCategoryID(request *materialCategoryIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse material category id: %w", err)
	}
	return id, nil
}

func toCreateMaterialCategoryCommand(request *CreateMaterialCategoryRequest) contract.CreateMaterialCategoryCommand {
	return contract.CreateMaterialCategoryCommand{Name: request.Name, Description: request.Description}
}

func toUpdateMaterialCategoryCommand(request *UpdateMaterialCategoryRequest) (uuid.UUID, contract.UpdateMaterialCategoryCommand, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, contract.UpdateMaterialCategoryCommand{}, fmt.Errorf("parse material category id: %w", err)
	}
	return id, contract.UpdateMaterialCategoryCommand{Name: request.Name, Description: request.Description}, nil
}

func toMaterialCategoryResponse(result *contract.MaterialCategoryResult) *MaterialCategoryResponse {
	if result == nil {
		return nil
	}
	return &MaterialCategoryResponse{ID: result.ID.String(), Name: result.Name, Description: result.Description, ArticleCount: result.ArticleCount}
}
