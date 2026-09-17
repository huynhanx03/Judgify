package contract

import (
	"time"

	"github.com/google/uuid"

	problemDto "github.com/huynhanx03/judgify/internal/problem/contract"
)

// MaterialResponse represents material article data in API response.
type MaterialResponse struct {
	ID                      uuid.UUID
	Slug                    string
	Title                   string
	Description             string
	Content                 string
	SanitizedHTML           string
	Difficulty              *problemDto.DifficultyResult
	Category                *MaterialCategoryResult
	Tags                    []*problemDto.TagResult
	AuthorID                uuid.UUID
	Status                  string
	Visibility              string
	ViewCount               int64
	EstimatedReadTime       int
	Version                 int64
	RevisionID              *uuid.UUID
	PublishedRevisionID     *uuid.UUID
	PublishedRevisionNumber *int64
	RevisionNumber          int64
	RenderArtifactID        *uuid.UUID
	RendererVersion         string
	SanitizerVersion        string
	PublishedAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// CreateMaterialCommand describes the data required to create a material article.
type CreateMaterialCommand struct {
	CommandID     uuid.UUID
	Slug          string
	Title         string
	Description   string
	Content       string
	DifficultyID  uuid.UUID
	CategoryID    uuid.UUID
	TagIDs        []uuid.UUID
	ChangeSummary string
	Status        string
	Visibility    string
}

// UpdateMaterialCommand describes a partial material article update.
type UpdateMaterialCommand struct {
	CommandID       uuid.UUID
	Title           *string
	Description     *string
	Content         *string
	DifficultyID    *uuid.UUID
	CategoryID      *uuid.UUID
	TagIDs          *[]uuid.UUID
	ExpectedVersion int64
	ChangeSummary   string
	Status          *string
	Visibility      *string
}

type MaterialSearchResponse struct {
	Records    []*MaterialResponse
	NextCursor string
}

type AdminMaterialSearchResponse struct {
	Records    []*MaterialResponse
	NextCursor string
}

type MaterialRevisionResponse struct {
	ID                       uuid.UUID
	MaterialID               uuid.UUID
	RevisionNumber           int64
	Title                    string
	Description              string
	ContentMarkdown          string
	SanitizedHTML            string
	CategoryID               uuid.UUID
	DifficultyID             uuid.UUID
	TagIDs                   []uuid.UUID
	AuthorID                 uuid.UUID
	EstimatedReadMinutes     int
	ChangeSummary            string
	CreatedAt                time.Time
	RenderSourceID           uuid.UUID
	RenderArtifactID         uuid.UUID
	NormalizedSourceChecksum string
	RendererVersion          string
	SanitizerPolicyVersion   string
}

type MaterialRevisionPage struct {
	Records    []*MaterialRevisionResponse
	NextCursor string
}

type MaterialLifecycleProjectionResponse struct {
	Version                int64       `json:"version"`
	Status                 string      `json:"status"`
	Visibility             string      `json:"visibility"`
	Slug                   string      `json:"slug"`
	RevisionID             *uuid.UUID  `json:"revision_id"`
	RevisionNumber         int64       `json:"revision_number"`
	Title                  string      `json:"title"`
	CategoryID             uuid.UUID   `json:"category_id"`
	DifficultyID           uuid.UUID   `json:"difficulty_id"`
	TagIDs                 []uuid.UUID `json:"tag_ids"`
	RenderSourceID         uuid.UUID   `json:"render_source_id"`
	RenderArtifactID       uuid.UUID   `json:"render_artifact_id"`
	SourceChecksum         string      `json:"source_checksum"`
	RendererVersion        string      `json:"renderer_version"`
	SanitizerPolicyVersion string      `json:"sanitizer_policy_version"`
}

type MaterialLifecyclePreviewResponse struct {
	Action                string
	MaterialID            uuid.UUID
	TargetRevisionID      *uuid.UUID
	ExpectedVersion       int64
	Before                MaterialLifecycleProjectionResponse
	After                 MaterialLifecycleProjectionResponse
	ConfirmationToken     string
	ConfirmationExpiresAt time.Time
}

// MaterialLifecycleReceipt is the canonical, replayable evidence returned by
// a committed lifecycle command. Material excludes Markdown and HTML.
type MaterialLifecycleReceipt struct {
	MaterialID       uuid.UUID
	Action           string
	CommandID        uuid.UUID
	EventID          uuid.UUID
	Version          int64
	After            MaterialLifecycleProjectionResponse
	CommittedAt      time.Time
	IdempotentReplay bool
}

// MaterialMutationReceipt is safe, durable evidence for an ordinary
// optimistic Material mutation. Rich content is reloaded through the query
// model and never copied into the idempotency receipt.
type MaterialMutationReceipt struct {
	MaterialID       uuid.UUID
	Action           string
	CommandID        uuid.UUID
	EventID          uuid.UUID
	Version          int64
	CommittedAt      time.Time
	IdempotentReplay bool
}

type MaterialSlugResponse struct {
	Material      *MaterialResponse
	Redirect      bool
	CanonicalSlug string
}
