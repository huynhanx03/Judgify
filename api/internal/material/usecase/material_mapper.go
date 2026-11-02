package usecase

import (
	"time"

	problemDto "github.com/huynhanx03/judgify/internal/problem/contract"
	problemEntity "github.com/huynhanx03/judgify/internal/problem/domain"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
)

// ToMaterialResponse converts entity to API response.
func ToMaterialResponse(e *entity.Material) *contract.MaterialResponse {
	if e == nil {
		return nil
	}
	resp := &contract.MaterialResponse{
		ID:                e.ID,
		Slug:              e.Slug,
		Title:             e.Title,
		Description:       e.Description,
		Content:           e.Content,
		AuthorID:          e.AuthorID,
		Status:            e.Status,
		Visibility:        e.Visibility,
		ViewCount:         e.ViewCount,
		EstimatedReadTime: e.EstimatedReadTime,
		Version:           e.Version,
		PublishedAt:       e.PublishedAt,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}
	if e.PublishedRevisionID != nil {
		value := *e.PublishedRevisionID
		resp.RevisionID = &value
		published := value
		resp.PublishedRevisionID = &published
	}
	if e.PublishedArtifactID != nil {
		value := *e.PublishedArtifactID
		resp.RenderArtifactID = &value
	}

	if e.Difficulty != nil {
		resp.Difficulty = &problemDto.DifficultyResult{
			ID:          e.Difficulty.ID,
			Name:        e.Difficulty.Name,
			Level:       e.Difficulty.Level,
			ExpReward:   e.Difficulty.ExpReward,
			Description: e.Difficulty.Description,
		}
	}

	if e.Category != nil {
		resp.Category = ToMaterialCategoryResult(e.Category)
	}

	if len(e.Tags) > 0 {
		resp.Tags = make([]*problemDto.TagResult, len(e.Tags))
		for i, t := range e.Tags {
			resp.Tags[i] = toProblemTagResponse(t)
		}
	}

	return resp
}

func toProblemTagResponse(tag problemEntity.Tag) *problemDto.TagResult {
	elements := make([]*problemDto.TagElementResult, len(tag.Elements))
	for index, element := range tag.Elements {
		elements[index] = &problemDto.TagElementResult{
			ID: element.ID, Name: element.Name, Code: element.Code,
		}
	}
	return &problemDto.TagResult{
		ID: tag.ID, Name: tag.Name, Elements: elements, Version: tag.Version,
	}
}

func ToPublicMaterialResponse(value entity.PublicMaterial, includeContent bool) *contract.MaterialResponse {
	revisionID := value.RevisionID
	artifactID := value.RenderArtifactID
	publishedAt := value.PublishedAt
	createdAt := value.CreatedAt
	if createdAt.IsZero() {
		createdAt = publishedAt
	}
	updatedAt := value.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = publishedAt
	}
	var publishedAtResponse *time.Time
	if !publishedAt.IsZero() &&
		(value.Status == "" || value.Status == entity.MaterialPublished ||
			value.Status == entity.MaterialArchived) {
		value := publishedAt
		publishedAtResponse = &value
	}
	response := &contract.MaterialResponse{
		ID: value.MaterialID, Slug: value.Slug, Title: value.Title,
		Description: value.Description, AuthorID: value.AuthorID,
		Status: string(value.Status), Visibility: string(value.Visibility),
		ViewCount: value.ViewCount, EstimatedReadTime: value.EstimatedReadMinutes,
		Version: value.Version, RevisionID: &revisionID,
		RevisionNumber: value.RevisionNumber, RenderArtifactID: &artifactID,
		RendererVersion: value.RendererVersion, SanitizerVersion: value.SanitizerPolicyVersion,
		PublishedAt: publishedAtResponse, CreatedAt: createdAt, UpdatedAt: updatedAt,
		Category: &contract.MaterialCategoryResult{ID: value.CategoryID, Name: value.CategoryName},
		Difficulty: &problemDto.DifficultyResult{
			ID: value.DifficultyID, Name: value.DifficultyName, Level: value.DifficultyLevel,
			Version: 1,
		},
	}
	if value.PublishedRevisionID != nil {
		publishedRevisionID := *value.PublishedRevisionID
		response.PublishedRevisionID = &publishedRevisionID
	}
	if value.PublishedRevisionNumber != nil {
		publishedRevisionNumber := *value.PublishedRevisionNumber
		response.PublishedRevisionNumber = &publishedRevisionNumber
	}
	if response.Status == "" {
		response.Status = string(entity.MaterialPublished)
	}
	if includeContent {
		response.SanitizedHTML = value.SanitizedHTML
	}
	if value.ContentMarkdown != "" {
		response.Content = value.ContentMarkdown
	}
	response.Tags = make([]*problemDto.TagResult, len(value.Tags))
	for index, tag := range value.Tags {
		response.Tags[index] = &problemDto.TagResult{
			ID: tag.ID, Name: tag.Name, Elements: []*problemDto.TagElementResult{}, Version: 1,
		}
	}
	return response
}
