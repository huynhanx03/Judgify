package http

import (
	"github.com/google/uuid"
	d "github.com/huynhanx03/go-common/pkg/dto"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	problemContract "github.com/huynhanx03/judgify/internal/problem/contract"
)

func toMaterialResponse(result *contract.MaterialResponse) *MaterialResponse {
	if result == nil {
		return nil
	}
	return &MaterialResponse{
		ID: result.ID.String(), Slug: result.Slug, Title: result.Title, Description: result.Description,
		Content: result.Content, SanitizedHTML: result.SanitizedHTML, Difficulty: toMaterialDifficultyDTO(result.Difficulty),
		Category: toMaterialCategoryResponse(result.Category), Tags: toMaterialTagDTOs(result.Tags),
		AuthorID: result.AuthorID.String(), Status: result.Status, Visibility: result.Visibility, ViewCount: result.ViewCount,
		EstimatedReadTime: result.EstimatedReadTime, Version: result.Version, RevisionID: formatMaterialUUID(result.RevisionID),
		PublishedRevisionID: formatMaterialUUID(result.PublishedRevisionID), PublishedRevisionNumber: result.PublishedRevisionNumber,
		RevisionNumber: result.RevisionNumber, RenderArtifactID: formatMaterialUUID(result.RenderArtifactID),
		RendererVersion: result.RendererVersion, SanitizerVersion: result.SanitizerVersion, PublishedAt: result.PublishedAt,
		CreatedAt: result.CreatedAt, UpdatedAt: result.UpdatedAt,
	}
}

func toMaterialPage(result *d.Paginated[*contract.MaterialResponse]) *d.Paginated[*MaterialResponse] {
	if result == nil {
		return nil
	}
	page := &d.Paginated[*MaterialResponse]{Pagination: result.Pagination}
	if result.Records == nil {
		return page
	}
	values := make([]*MaterialResponse, len(*result.Records))
	for index, record := range *result.Records {
		values[index] = toMaterialResponse(record)
	}
	page.Records = &values
	return page
}

func toMaterialSearchResponse(result *contract.MaterialSearchResponse) *materialSearchResponse {
	if result == nil {
		return nil
	}
	return &materialSearchResponse{Records: toMaterialResponses(result.Records), NextCursor: result.NextCursor}
}

func toAdminMaterialSearchResponse(result *contract.AdminMaterialSearchResponse) *adminMaterialSearchResponse {
	if result == nil {
		return nil
	}
	return &adminMaterialSearchResponse{Records: toMaterialResponses(result.Records), NextCursor: result.NextCursor}
}

func toMaterialSlugResponse(result *contract.MaterialSlugResponse) *materialSlugResponse {
	if result == nil {
		return nil
	}
	return &materialSlugResponse{Material: toMaterialResponse(result.Material), Redirect: result.Redirect, CanonicalSlug: result.CanonicalSlug}
}

func toMaterialResponses(results []*contract.MaterialResponse) []*MaterialResponse {
	if results == nil {
		return nil
	}
	values := make([]*MaterialResponse, len(results))
	for index, result := range results {
		values[index] = toMaterialResponse(result)
	}
	return values
}

func toMaterialRevisionResponse(result *contract.MaterialRevisionResponse) *materialRevisionResponse {
	if result == nil {
		return nil
	}
	return &materialRevisionResponse{
		ID: result.ID.String(), MaterialID: result.MaterialID.String(), RevisionNumber: result.RevisionNumber,
		Title: result.Title, Description: result.Description, ContentMarkdown: result.ContentMarkdown, SanitizedHTML: result.SanitizedHTML,
		CategoryID: result.CategoryID.String(), DifficultyID: result.DifficultyID.String(), TagIDs: formatMaterialUUIDs(result.TagIDs),
		AuthorID: result.AuthorID.String(), EstimatedReadMinutes: result.EstimatedReadMinutes, ChangeSummary: result.ChangeSummary,
		CreatedAt: result.CreatedAt, RenderSourceID: result.RenderSourceID.String(), RenderArtifactID: result.RenderArtifactID.String(),
		NormalizedSourceChecksum: result.NormalizedSourceChecksum, RendererVersion: result.RendererVersion,
		SanitizerPolicyVersion: result.SanitizerPolicyVersion,
	}
}

func toMaterialRevisionPage(result *contract.MaterialRevisionPage) *materialRevisionPage {
	if result == nil {
		return nil
	}
	page := &materialRevisionPage{NextCursor: result.NextCursor}
	if result.Records != nil {
		page.Records = make([]*materialRevisionResponse, len(result.Records))
		for index, record := range result.Records {
			page.Records[index] = toMaterialRevisionResponse(record)
		}
	}
	return page
}

func formatMaterialUUIDs(values []uuid.UUID) []string {
	if values == nil {
		return nil
	}
	formatted := make([]string, len(values))
	for index, value := range values {
		formatted[index] = value.String()
	}
	return formatted
}

func toMaterialLifecycleProjectionResponse(result contract.MaterialLifecycleProjectionResponse) materialLifecycleProjectionResponse {
	return materialLifecycleProjectionResponse{
		Version: result.Version, Status: result.Status, Visibility: result.Visibility, Slug: result.Slug,
		RevisionID: formatMaterialUUID(result.RevisionID), RevisionNumber: result.RevisionNumber, Title: result.Title,
		CategoryID: result.CategoryID.String(), DifficultyID: result.DifficultyID.String(), TagIDs: formatMaterialUUIDs(result.TagIDs),
		RenderSourceID: result.RenderSourceID.String(), RenderArtifactID: result.RenderArtifactID.String(),
		SourceChecksum: result.SourceChecksum, RendererVersion: result.RendererVersion, SanitizerPolicyVersion: result.SanitizerPolicyVersion,
	}
}

func toMaterialLifecyclePreviewResponse(result *contract.MaterialLifecyclePreviewResponse) *materialLifecyclePreviewResponse {
	if result == nil {
		return nil
	}
	return &materialLifecyclePreviewResponse{
		Action: result.Action, MaterialID: result.MaterialID.String(), TargetRevisionID: formatMaterialUUID(result.TargetRevisionID),
		ExpectedVersion: result.ExpectedVersion, Before: toMaterialLifecycleProjectionResponse(result.Before),
		After: toMaterialLifecycleProjectionResponse(result.After), ConfirmationToken: result.ConfirmationToken,
		ConfirmationExpiresAt: result.ConfirmationExpiresAt,
	}
}

func toMaterialLifecycleReceipt(result *contract.MaterialLifecycleReceipt) *materialLifecycleReceipt {
	if result == nil {
		return nil
	}
	return &materialLifecycleReceipt{
		MaterialID: result.MaterialID.String(), Action: result.Action, CommandID: result.CommandID.String(), EventID: result.EventID.String(),
		Version: result.Version, After: toMaterialLifecycleProjectionResponse(result.After), CommittedAt: result.CommittedAt,
		IdempotentReplay: result.IdempotentReplay,
	}
}

func toMaterialMutationReceipt(result *contract.MaterialMutationReceipt) *materialMutationReceipt {
	if result == nil {
		return nil
	}
	return &materialMutationReceipt{
		MaterialID: result.MaterialID.String(), Action: result.Action, CommandID: result.CommandID.String(), EventID: result.EventID.String(),
		Version: result.Version, CommittedAt: result.CommittedAt, IdempotentReplay: result.IdempotentReplay,
	}
}

func toMaterialDifficultyDTO(result *problemContract.DifficultyResult) *materialDifficultyDTO {
	if result == nil {
		return nil
	}
	return &materialDifficultyDTO{ID: result.ID.String(), Name: result.Name, Level: result.Level, ExpReward: result.ExpReward, Description: result.Description, Version: result.Version}
}

func toMaterialTagDTOs(results []*problemContract.TagResult) []*materialTagDTO {
	if results == nil {
		return nil
	}
	values := make([]*materialTagDTO, len(results))
	for index, result := range results {
		values[index] = toMaterialTagDTO(result)
	}
	return values
}

func toMaterialTagDTO(result *problemContract.TagResult) *materialTagDTO {
	if result == nil {
		return nil
	}
	value := &materialTagDTO{ID: result.ID.String(), Name: result.Name, Version: result.Version}
	if result.Elements != nil {
		value.Elements = make([]*materialTagElementDTO, len(result.Elements))
		for index, element := range result.Elements {
			if element != nil {
				value.Elements[index] = &materialTagElementDTO{ID: element.ID.String(), Name: element.Name, Code: element.Code}
			}
		}
	}
	return value
}

func formatMaterialUUID(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	formatted := value.String()
	return &formatted
}
