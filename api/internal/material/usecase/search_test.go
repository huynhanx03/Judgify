package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	entity "github.com/huynhanx03/judgify/internal/material/domain"
	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	"github.com/huynhanx03/judgify/internal/ports"
)

type searchRepositoryStub struct {
	query    ports.SearchQuery
	result   ports.SearchResult
	revision entity.MaterialRevision
	admin    entity.PublicMaterial
}

func (stub *searchRepositoryStub) SearchPublic(_ context.Context, query ports.SearchQuery) (ports.SearchResult, error) {
	stub.query = query
	return stub.result, nil
}
func (stub *searchRepositoryStub) SearchAdmin(_ context.Context, query ports.SearchQuery) (ports.SearchResult, error) {
	stub.query = query
	return stub.result, nil
}
func (*searchRepositoryStub) ResolvePublicSlug(context.Context, string, bool) (entity.SlugResolution, error) {
	return entity.SlugResolution{}, nil
}
func (*searchRepositoryStub) RelatedPublic(context.Context, uuid.UUID, bool, int) ([]entity.PublicMaterial, error) {
	return nil, nil
}
func (*searchRepositoryStub) ListRevisions(context.Context, uuid.UUID, int64, int) ([]entity.MaterialRevisionSummary, error) {
	return nil, nil
}
func (stub *searchRepositoryStub) LoadRevision(context.Context, uuid.UUID, uuid.UUID) (entity.MaterialRevision, error) {
	return stub.revision, nil
}
func (stub *searchRepositoryStub) LoadAdminProjection(context.Context, uuid.UUID) (entity.PublicMaterial, error) {
	return stub.admin, nil
}

func TestSearchAppliesBoundedDefaultsAndClosedSort(t *testing.T) {
	repository := &searchRepositoryStub{}
	service, err := NewSearchService(repository)
	require.NoError(t, err)

	_, err = service.Search(context.Background(), ports.SearchQuery{Text: "  cây đoạn  "})
	require.NoError(t, err)
	require.Equal(t, "cây đoạn", repository.query.Text)
	require.Equal(t, "relevance", repository.query.Sort)
	require.Equal(t, 24, repository.query.Limit)

	_, err = service.Search(context.Background(), ports.SearchQuery{Sort: "random"})
	require.ErrorIs(t, err, entity.ErrMaterialInvalidCommand)
}

func TestSearchNeverRequestsHiddenProjectionForAnonymousCaller(t *testing.T) {
	repository := &searchRepositoryStub{}
	service, err := NewSearchService(repository)
	require.NoError(t, err)
	_, err = service.Search(context.Background(), ports.SearchQuery{Authenticated: false})
	require.NoError(t, err)
	require.False(t, repository.query.Authenticated)
}

func TestGetAdminReturnsTheCurrentDraftAndPublishedRevisionEvidence(t *testing.T) {
	materialID := uuid.New()
	draftRevisionID := uuid.New()
	publishedRevisionID := uuid.New()
	publishedRevisionNumber := int64(3)
	repository := &searchRepositoryStub{admin: entity.PublicMaterial{
		MaterialID:              materialID,
		Slug:                    "cay-fenwick",
		Title:                   "Cây Fenwick nâng cao",
		Description:             "Bản nháp mới",
		SanitizedHTML:           "<p>Bản nháp mới</p>",
		ContentMarkdown:         "# Bản nháp mới",
		CategoryID:              uuid.New(),
		CategoryName:            "Cấu trúc dữ liệu",
		DifficultyID:            uuid.New(),
		DifficultyName:          "Nâng cao",
		DifficultyLevel:         4,
		AuthorID:                uuid.New(),
		EstimatedReadMinutes:    2,
		PublishedAt:             time.Now().UTC(),
		Version:                 8,
		RevisionID:              draftRevisionID,
		RevisionNumber:          4,
		PublishedRevisionID:     &publishedRevisionID,
		PublishedRevisionNumber: &publishedRevisionNumber,
		RenderArtifactID:        uuid.New(),
		RendererVersion:         "renderer-v1",
		SanitizerPolicyVersion:  "sanitizer-v1",
		Visibility:              entity.MaterialPublic,
		Status:                  entity.MaterialPublished,
	}}
	query, err := NewSearchService(repository)
	require.NoError(t, err)

	response, err := query.GetAdmin(context.Background(), materialID)
	require.NoError(t, err)
	require.Equal(t, draftRevisionID, *response.RevisionID)
	require.Equal(t, publishedRevisionID, *response.PublishedRevisionID)
	require.Equal(t, publishedRevisionNumber, *response.PublishedRevisionNumber)
	require.Equal(t, repository.admin.ContentMarkdown, response.Content)
	require.Equal(t, repository.admin.SanitizedHTML, response.SanitizedHTML)
}

type revisionContentRepositoryStub struct {
	source   contentrender.Source
	artifact contentrender.Artifact
}

func (*revisionContentRepositoryStub) RegisterSource(context.Context, contentrender.Source) (contentrender.Source, error) {
	return contentrender.Source{}, contentrender.ErrSourceAlreadyExists
}

func (stub *revisionContentRepositoryStub) GetSource(_ context.Context, id uuid.UUID) (contentrender.Source, error) {
	if id != stub.source.ID {
		return contentrender.Source{}, contentrender.ErrSourceNotFound
	}
	return stub.source, nil
}

func (*revisionContentRepositoryStub) StoreArtifact(context.Context, contentrender.Source, contentrender.Artifact) (contentrender.Artifact, error) {
	return contentrender.Artifact{}, contentrender.ErrArtifactAlreadyExists
}

func (stub *revisionContentRepositoryStub) GetArtifact(_ context.Context, id uuid.UUID) (contentrender.Artifact, error) {
	if id != stub.artifact.ID {
		return contentrender.Artifact{}, contentrender.ErrArtifactNotFound
	}
	return stub.artifact, nil
}

func (*revisionContentRepositoryStub) FindArtifact(context.Context, uuid.UUID, string, string) (contentrender.Artifact, error) {
	return contentrender.Artifact{}, contentrender.ErrArtifactNotFound
}

func TestGetRevisionReturnsOnlyArtifactBoundToTheExactImmutableSource(t *testing.T) {
	ctx := context.Background()
	materialID, revisionID := uuid.New(), uuid.New()
	sourceID, artifactID := uuid.New(), uuid.New()
	normalized, err := contentrender.NormalizeMarkdown([]byte("# Safe material\n"))
	require.NoError(t, err)
	source := contentrender.Source{
		ID: sourceID, OwnerRevisionID: revisionID, Kind: contentrender.MaterialBody,
		Checksum: normalized.Checksum, CreatedAt: time.Now().UTC(),
	}
	revision := entity.MaterialRevision{
		ID: revisionID, MaterialID: materialID, RevisionNumber: 1,
		Title: "Safe material", Description: "A safe material revision",
		ContentMarkdown: normalized.Bytes, CategoryID: uuid.New(), DifficultyID: uuid.New(),
		EstimatedReadMinutes: 1, AuthorID: uuid.New(), CreatedAt: time.Now().UTC(),
		ChangeSummary: "initial material revision", NormalizedSourceChecksum: normalized.Checksum,
		RenderSourceID: sourceID, RenderSourceKind: contentrender.MaterialBody,
		RenderArtifactID: artifactID,
	}
	renderer, err := contentrender.NewRenderer()
	require.NoError(t, err)
	exactArtifact, err := renderer.Render(ctx, contentrender.RenderRequest{
		SourceID: source.ID, SourceKind: source.Kind, SourceChecksum: source.Checksum,
		NormalizedMarkdown: normalized.Bytes,
	})
	require.NoError(t, err)
	exactArtifact.ID = artifactID

	for _, test := range []struct {
		name      string
		artifact  contentrender.Artifact
		wantError bool
	}{
		{name: "exact source tuple", artifact: exactArtifact},
		{name: "artifact from another source", artifact: func() contentrender.Artifact {
			mismatched, renderErr := renderer.Render(ctx, contentrender.RenderRequest{
				SourceID: uuid.New(), SourceKind: source.Kind, SourceChecksum: source.Checksum,
				NormalizedMarkdown: normalized.Bytes,
			})
			require.NoError(t, renderErr)
			mismatched.ID = artifactID
			return mismatched
		}(), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &searchRepositoryStub{revision: revision}
			content := &revisionContentRepositoryStub{source: source, artifact: test.artifact}
			query, serviceErr := NewSearchService(repository, content)
			require.NoError(t, serviceErr)

			response, serviceErr := query.GetRevision(ctx, materialID, revisionID)
			if test.wantError {
				require.Nil(t, response)
				require.ErrorIs(t, serviceErr, entity.ErrMaterialArtifactRejected)
				return
			}
			require.NoError(t, serviceErr)
			require.Equal(t, exactArtifact.SanitizedHTML, response.SanitizedHTML)
		})
	}
}

var _ contentrender.Repository = (*revisionContentRepositoryStub)(nil)
