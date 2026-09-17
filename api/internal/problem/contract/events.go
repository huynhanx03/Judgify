package contract

import "github.com/google/uuid"

const (
	AggregateProblem = "problem"

	EventDraftSavedV1 = "problem.draft_saved.v1"
	EventPublishedV1  = "problem.published.v1"
	EventArchivedV1   = "problem.archived.v1"
)

// DraftSavedV1 is a content-free immutable revision pointer. Checksums let
// downstream consumers detect corrupt or mismatched revision projections
// without transporting statements, tests, checker config, or reward facts.
type DraftSavedV1 struct {
	ProblemID              uuid.UUID `json:"problem_id"`
	ProblemRevisionID      uuid.UUID `json:"problem_revision_id"`
	TestsetRevisionID      uuid.UUID `json:"testset_revision_id"`
	RenderSourceID         uuid.UUID `json:"render_source_id"`
	RenderArtifactID       uuid.UUID `json:"render_artifact_id"`
	ArtifactStatus         string    `json:"artifact_status"`
	ContentChecksum        string    `json:"content_checksum"`
	TestsetChecksum        string    `json:"testset_checksum"`
	RewardSnapshotChecksum string    `json:"reward_snapshot_checksum"`
	TestCount              int       `json:"test_count"`
	Version                int64     `json:"version"`
}

type PublishedV1 struct {
	ProblemID              uuid.UUID `json:"problem_id"`
	ProblemRevisionID      uuid.UUID `json:"problem_revision_id"`
	TestsetRevisionID      uuid.UUID `json:"testset_revision_id"`
	RenderSourceID         uuid.UUID `json:"render_source_id"`
	RenderArtifactID       uuid.UUID `json:"render_artifact_id"`
	ContentChecksum        string    `json:"content_checksum"`
	TestsetChecksum        string    `json:"testset_checksum"`
	RewardSnapshotChecksum string    `json:"reward_snapshot_checksum"`
	Version                int64     `json:"version"`
}

type ArchivedV1 struct {
	ProblemID           uuid.UUID `json:"problem_id"`
	PublishedRevisionID uuid.UUID `json:"published_revision_id"`
	Version             int64     `json:"version"`
}
