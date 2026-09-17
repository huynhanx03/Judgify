package usecase

import (
	"context"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/material/contract"
	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/ports"
)

type searchService struct {
	repository ports.MaterialQueryRepository
	content    contentrender.Repository
}

func NewSearchService(repository ports.MaterialQueryRepository, content ...contentrender.Repository) (ports.MaterialQueryService, error) {
	if repository == nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	service := &searchService{repository: repository}
	if len(content) == 1 {
		service.content = content[0]
	} else if len(content) > 1 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	return service, nil
}

func (service *searchService) Search(ctx context.Context, query ports.SearchQuery) (*contract.MaterialSearchResponse, error) {
	return service.search(ctx, query, false)
}

func (service *searchService) SearchAdmin(ctx context.Context, query ports.SearchQuery) (*contract.AdminMaterialSearchResponse, error) {
	query.Text = strings.TrimSpace(query.Text)
	if invalidSearchIdentity(query) ||
		query.Sort != "updated" ||
		query.Limit < 0 || query.Limit > entity.MaximumMaterialPageSize {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if query.Status != nil &&
		*query.Status != entity.MaterialDraft &&
		*query.Status != entity.MaterialInReview &&
		*query.Status != entity.MaterialPublished &&
		*query.Status != entity.MaterialArchived {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if query.Limit == 0 {
		query.Limit = 24
	}
	result, err := service.repository.SearchAdmin(ctx, query)
	if err != nil {
		return nil, err
	}
	response := &contract.AdminMaterialSearchResponse{
		Records:    make([]*contract.MaterialResponse, len(result.Records)),
		NextCursor: result.NextCursor,
	}
	for index, item := range result.Records {
		response.Records[index] = ToPublicMaterialResponse(item, false)
	}
	return response, nil
}

func (service *searchService) GetAdmin(
	ctx context.Context,
	materialID uuid.UUID,
) (*contract.MaterialResponse, error) {
	if materialID == uuid.Nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	material, err := service.repository.LoadAdminProjection(ctx, materialID)
	if err != nil {
		return nil, err
	}
	return ToPublicMaterialResponse(material, true), nil
}

func (service *searchService) search(ctx context.Context, query ports.SearchQuery, admin bool) (*contract.MaterialSearchResponse, error) {
	query.Text = strings.TrimSpace(query.Text)
	if invalidSearchIdentity(query) ||
		(query.Limit < 0 || query.Limit > entity.MaximumMaterialPageSize) {
		return nil, entity.ErrMaterialInvalidCommand
	}
	if query.Limit == 0 {
		query.Limit = 24
	}
	if query.Sort == "" {
		if query.Text == "" {
			query.Sort = "published"
		} else {
			query.Sort = "relevance"
		}
	}
	if query.Sort != "relevance" && query.Sort != "published" && query.Sort != "popularity" {
		return nil, entity.ErrMaterialInvalidCommand
	}
	var result ports.SearchResult
	var err error
	if admin {
		return nil, entity.ErrMaterialInvalidCommand
	} else {
		result, err = service.repository.SearchPublic(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	response := &contract.MaterialSearchResponse{
		Records:    make([]*contract.MaterialResponse, len(result.Records)),
		NextCursor: result.NextCursor,
	}
	for index, item := range result.Records {
		response.Records[index] = ToPublicMaterialResponse(item, false)
	}
	return response, nil
}

func invalidSearchIdentity(query ports.SearchQuery) bool {
	if len(query.Text) > entity.MaximumMaterialSearchBytes ||
		len(query.TagIDs) > entity.MaximumMaterialTags ||
		(query.CategoryID != nil && *query.CategoryID == uuid.Nil) ||
		(query.DifficultyID != nil && *query.DifficultyID == uuid.Nil) {
		return true
	}
	seen := make(map[uuid.UUID]struct{}, len(query.TagIDs))
	for _, tagID := range query.TagIDs {
		if tagID == uuid.Nil {
			return true
		}
		if _, duplicate := seen[tagID]; duplicate {
			return true
		}
		seen[tagID] = struct{}{}
	}
	return false
}

func (service *searchService) ListRevisions(
	ctx context.Context,
	materialID uuid.UUID,
	cursor string,
	limit int,
) (*contract.MaterialRevisionPage, error) {
	if materialID == uuid.Nil || len(cursor) > 32 || limit < 0 || limit > 100 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	before := int64(0)
	if cursor != "" {
		value, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || value < 2 {
			return nil, entity.ErrMaterialInvalidCommand
		}
		before = value
	}
	if limit == 0 {
		limit = 25
	}
	revisions, err := service.repository.ListRevisions(ctx, materialID, before, limit+1)
	if err != nil {
		return nil, err
	}
	if len(revisions) == 0 && before == 0 {
		if _, err := service.repository.LoadAdminProjection(ctx, materialID); err != nil {
			return nil, err
		}
	}
	page := &contract.MaterialRevisionPage{
		Records: make([]*contract.MaterialRevisionResponse, min(len(revisions), limit)),
	}
	for index := range page.Records {
		page.Records[index] = revisionSummaryResponse(revisions[index])
	}
	if len(revisions) > limit {
		page.NextCursor = strconv.FormatInt(revisions[limit-1].RevisionNumber, 10)
	}
	return page, nil
}

func revisionSummaryResponse(
	revision entity.MaterialRevisionSummary,
) *contract.MaterialRevisionResponse {
	return &contract.MaterialRevisionResponse{
		ID: revision.ID, MaterialID: revision.MaterialID,
		RevisionNumber: revision.RevisionNumber, Title: revision.Title,
		Description: revision.Description, CategoryID: revision.CategoryID,
		DifficultyID:         revision.DifficultyID,
		TagIDs:               append([]uuid.UUID(nil), revision.TagIDs...),
		AuthorID:             revision.AuthorID,
		EstimatedReadMinutes: revision.EstimatedReadMinutes,
		ChangeSummary:        revision.ChangeSummary, CreatedAt: revision.CreatedAt,
		RenderSourceID:   revision.RenderSourceID,
		RenderArtifactID: revision.RenderArtifactID,
		NormalizedSourceChecksum: hex.EncodeToString(
			revision.NormalizedSourceChecksum[:],
		),
	}
}

func (service *searchService) GetRevision(
	ctx context.Context,
	materialID uuid.UUID,
	revisionID uuid.UUID,
) (*contract.MaterialRevisionResponse, error) {
	if materialID == uuid.Nil || revisionID == uuid.Nil || service.content == nil {
		return nil, entity.ErrMaterialInvalidCommand
	}
	revision, err := service.repository.LoadRevision(ctx, materialID, revisionID)
	if err != nil {
		return nil, err
	}
	artifact, err := loadValidatedRevisionArtifact(ctx, service.content, revision, false)
	if err != nil {
		return nil, err
	}
	return revisionResponse(revision, artifact, true), nil
}

func revisionResponse(
	revision entity.MaterialRevision,
	artifact contentrender.Artifact,
	includeContent bool,
) *contract.MaterialRevisionResponse {
	response := &contract.MaterialRevisionResponse{
		ID: revision.ID, MaterialID: revision.MaterialID,
		RevisionNumber: revision.RevisionNumber, Title: revision.Title,
		Description: revision.Description, CategoryID: revision.CategoryID,
		DifficultyID:         revision.DifficultyID,
		TagIDs:               append([]uuid.UUID(nil), revision.TagIDs...),
		AuthorID:             revision.AuthorID,
		EstimatedReadMinutes: revision.EstimatedReadMinutes,
		ChangeSummary:        revision.ChangeSummary, CreatedAt: revision.CreatedAt,
		RenderSourceID:           revision.RenderSourceID,
		RenderArtifactID:         revision.RenderArtifactID,
		NormalizedSourceChecksum: hex.EncodeToString(revision.NormalizedSourceChecksum[:]),
	}
	if includeContent {
		response.ContentMarkdown = string(revision.ContentMarkdown)
		response.SanitizedHTML = artifact.SanitizedHTML
		response.RendererVersion = artifact.RendererVersion
		response.SanitizerPolicyVersion = artifact.SanitizerPolicyVersion
	}
	return response
}

func (service *searchService) Resolve(ctx context.Context, slug string, authenticated bool) (*contract.MaterialSlugResponse, error) {
	normalized, err := entity.NormalizeSlug(slug)
	if err != nil {
		return nil, err
	}
	resolution, err := service.repository.ResolvePublicSlug(ctx, normalized, authenticated)
	if err != nil {
		return nil, err
	}
	return &contract.MaterialSlugResponse{
		Material: ToPublicMaterialResponse(resolution.Material, true),
		Redirect: resolution.Redirect, CanonicalSlug: resolution.CanonicalSlug,
	}, nil
}

func (service *searchService) Related(ctx context.Context, slug string, authenticated bool, limit int) ([]*contract.MaterialResponse, error) {
	normalized, err := entity.NormalizeSlug(slug)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = 6
	}
	if limit < 1 || limit > 20 {
		return nil, entity.ErrMaterialInvalidCommand
	}
	resolution, err := service.repository.ResolvePublicSlug(ctx, normalized, authenticated)
	if err != nil {
		return nil, err
	}
	records, err := service.repository.RelatedPublic(ctx, resolution.Material.MaterialID, authenticated, limit)
	if err != nil {
		return nil, err
	}
	response := make([]*contract.MaterialResponse, len(records))
	for index, item := range records {
		response[index] = ToPublicMaterialResponse(item, false)
	}
	return response, nil
}

var _ ports.MaterialQueryService = (*searchService)(nil)
