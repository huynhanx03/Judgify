package http

import "time"

// MaterialResponse is the HTTP representation of a material read model.
type MaterialResponse struct {
	ID                      string                    `json:"id"`
	Slug                    string                    `json:"slug"`
	Title                   string                    `json:"title"`
	Description             string                    `json:"description"`
	Content                 string                    `json:"content,omitempty"`
	SanitizedHTML           string                    `json:"sanitized_html,omitempty"`
	Difficulty              *materialDifficultyDTO    `json:"difficulty"`
	Category                *MaterialCategoryResponse `json:"category"`
	Tags                    []*materialTagDTO         `json:"tags"`
	AuthorID                string                    `json:"author_id"`
	Status                  string                    `json:"status"`
	Visibility              string                    `json:"visibility"`
	ViewCount               int64                     `json:"view_count"`
	EstimatedReadTime       int                       `json:"estimated_read_time"`
	Version                 int64                     `json:"version"`
	RevisionID              *string                   `json:"revision_id,omitempty"`
	PublishedRevisionID     *string                   `json:"published_revision_id,omitempty"`
	PublishedRevisionNumber *int64                    `json:"published_revision_number,omitempty"`
	RevisionNumber          int64                     `json:"revision_number,omitempty"`
	RenderArtifactID        *string                   `json:"render_artifact_id,omitempty"`
	RendererVersion         string                    `json:"renderer_version,omitempty"`
	SanitizerVersion        string                    `json:"sanitizer_policy_version,omitempty"`
	PublishedAt             *time.Time                `json:"published_at,omitempty"`
	CreatedAt               time.Time                 `json:"created_at"`
	UpdatedAt               time.Time                 `json:"updated_at"`
}

type materialDifficultyDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	ExpReward   int64  `json:"exp_reward"`
	Description string `json:"description,omitempty"`
	Version     int64  `json:"version"`
}

type materialTagDTO struct {
	ID       string                   `json:"id"`
	Name     string                   `json:"name"`
	Elements []*materialTagElementDTO `json:"elements"`
	Version  int64                    `json:"version"`
}

type materialTagElementDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type materialSearchResponse struct {
	Records    []*MaterialResponse `json:"records"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type adminMaterialSearchResponse struct {
	Records    []*MaterialResponse `json:"records"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

type materialSlugResponse struct {
	Material      *MaterialResponse `json:"material"`
	Redirect      bool              `json:"redirect"`
	CanonicalSlug string            `json:"canonical_slug"`
}

type materialRevisionResponse struct {
	ID                       string    `json:"id"`
	MaterialID               string    `json:"material_id"`
	RevisionNumber           int64     `json:"revision_number"`
	Title                    string    `json:"title"`
	Description              string    `json:"description"`
	ContentMarkdown          string    `json:"content_markdown,omitempty"`
	SanitizedHTML            string    `json:"sanitized_html,omitempty"`
	CategoryID               string    `json:"category_id"`
	DifficultyID             string    `json:"difficulty_id"`
	TagIDs                   []string  `json:"tag_ids"`
	AuthorID                 string    `json:"author_id"`
	EstimatedReadMinutes     int       `json:"estimated_read_minutes"`
	ChangeSummary            string    `json:"change_summary"`
	CreatedAt                time.Time `json:"created_at"`
	RenderSourceID           string    `json:"render_source_id"`
	RenderArtifactID         string    `json:"render_artifact_id"`
	NormalizedSourceChecksum string    `json:"normalized_source_checksum"`
	RendererVersion          string    `json:"renderer_version,omitempty"`
	SanitizerPolicyVersion   string    `json:"sanitizer_policy_version,omitempty"`
}

type materialRevisionPage struct {
	Records    []*materialRevisionResponse `json:"records"`
	NextCursor string                      `json:"next_cursor,omitempty"`
}

type materialLifecycleProjectionResponse struct {
	Version                int64    `json:"version"`
	Status                 string   `json:"status"`
	Visibility             string   `json:"visibility"`
	Slug                   string   `json:"slug"`
	RevisionID             *string  `json:"revision_id,omitempty"`
	RevisionNumber         int64    `json:"revision_number,omitempty"`
	Title                  string   `json:"title,omitempty"`
	CategoryID             string   `json:"category_id"`
	DifficultyID           string   `json:"difficulty_id"`
	TagIDs                 []string `json:"tag_ids"`
	RenderSourceID         string   `json:"render_source_id"`
	RenderArtifactID       string   `json:"render_artifact_id"`
	SourceChecksum         string   `json:"source_checksum"`
	RendererVersion        string   `json:"renderer_version"`
	SanitizerPolicyVersion string   `json:"sanitizer_policy_version"`
}

type materialLifecyclePreviewResponse struct {
	Action                string                              `json:"action"`
	MaterialID            string                              `json:"material_id"`
	TargetRevisionID      *string                             `json:"target_revision_id,omitempty"`
	ExpectedVersion       int64                               `json:"expected_version"`
	Before                materialLifecycleProjectionResponse `json:"before"`
	After                 materialLifecycleProjectionResponse `json:"after"`
	ConfirmationToken     string                              `json:"confirmation_token"`
	ConfirmationExpiresAt time.Time                           `json:"confirmation_expires_at"`
}

type materialLifecycleReceipt struct {
	MaterialID       string                              `json:"material_id"`
	Action           string                              `json:"action"`
	CommandID        string                              `json:"command_id"`
	EventID          string                              `json:"event_id"`
	Version          int64                               `json:"version"`
	After            materialLifecycleProjectionResponse `json:"after"`
	CommittedAt      time.Time                           `json:"committed_at"`
	IdempotentReplay bool                                `json:"idempotent_replay"`
}

type materialMutationReceipt struct {
	MaterialID       string    `json:"material_id"`
	Action           string    `json:"action"`
	CommandID        string    `json:"command_id"`
	EventID          string    `json:"event_id"`
	Version          int64     `json:"version"`
	CommittedAt      time.Time `json:"committed_at"`
	IdempotentReplay bool      `json:"idempotent_replay"`
}
