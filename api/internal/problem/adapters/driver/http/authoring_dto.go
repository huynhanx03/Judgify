package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SaveProblemDraftRequest is the HTTP payload for creating an immutable draft revision.
type SaveProblemDraftRequest struct {
	ID                 string                      `json:"-" uri:"id" validate:"required,uuid"`
	ExpectedVersion    int64                       `json:"expected_version" validate:"required,min=1"`
	CatalogVersion     string                      `json:"catalog_version" validate:"required,len=67"`
	Title              string                      `json:"title" validate:"required,max=300"`
	StatementMarkdown  string                      `json:"statement_markdown" validate:"required"`
	DifficultyID       string                      `json:"difficulty_id" validate:"required,uuid"`
	TagIDs             []string                    `json:"tag_ids"`
	CPUTimeMs          int                         `json:"cpu_time_ms" validate:"required,min=1"`
	WallTimeMs         int                         `json:"wall_time_ms" validate:"required,min=1"`
	MemoryLimitKb      int                         `json:"memory_limit_kb" validate:"required,min=1"`
	OutputLimitBytes   int                         `json:"output_limit_bytes" validate:"required,min=1"`
	ProcessLimit       int                         `json:"process_limit" validate:"required,min=1"`
	CheckerKey         string                      `json:"checker_key" validate:"required"`
	CheckerVersion     string                      `json:"checker_version" validate:"required"`
	CheckerConfig      json.RawMessage             `json:"checker_config" validate:"required"`
	AllowedRuntimeKeys []string                    `json:"allowed_runtime_keys" validate:"required,min=1"`
	Groups             []DraftTestcaseGroupRequest `json:"groups" validate:"required,min=1"`
	Reason             string                      `json:"reason" validate:"required,max=1024"`
}

type DraftTestcaseGroupRequest struct {
	StopOnFailure bool                   `json:"stop_on_failure"`
	Cases         []DraftTestcaseRequest `json:"cases" validate:"required,min=1"`
}

type DraftTestcaseRequest struct {
	TestCaseID     *string `json:"test_case_id,omitempty" validate:"omitempty,uuid"`
	Kind           string  `json:"kind" validate:"required"`
	Input          string  `json:"input"`
	ExpectedOutput string  `json:"expected_output"`
}

// PublishProblemRequest is the HTTP payload for publishing a draft.
type PublishProblemRequest struct {
	ID                string `json:"-" uri:"id" validate:"required,uuid"`
	DraftRevisionID   string `json:"draft_revision_id" validate:"required,uuid"`
	TestsetRevisionID string `json:"testset_revision_id" validate:"required,uuid"`
	ExpectedVersion   int64  `json:"expected_version" validate:"required,min=1"`
	CatalogVersion    string `json:"catalog_version" validate:"required,len=67"`
	Reason            string `json:"reason" validate:"required,max=1024"`
}

// ArchiveProblemRequest is the HTTP payload for archiving a problem.
type ArchiveProblemRequest struct {
	ID              string `json:"-" uri:"id" validate:"required,uuid"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,min=1"`
	Reason          string `json:"reason" validate:"required,max=1024"`
}

type ProblemDraftResponse struct {
	ProblemID           string    `json:"problem_id"`
	RevisionID          string    `json:"revision_id"`
	TestsetRevisionID   string    `json:"testset_revision_id"`
	Version             int64     `json:"version"`
	ArtifactStatus      string    `json:"artifact_status"`
	ArtifactDiagnostics []string  `json:"artifact_diagnostics"`
	SavedAt             time.Time `json:"saved_at"`
}

type ProblemAuthoringDraftResponse struct {
	ProblemID           string                           `json:"problem_id"`
	Slug                string                           `json:"slug"`
	Lifecycle           string                           `json:"lifecycle"`
	Version             int64                            `json:"version"`
	RevisionID          string                           `json:"revision_id"`
	TestsetRevisionID   string                           `json:"testset_revision_id"`
	Title               string                           `json:"title"`
	StatementMarkdown   string                           `json:"statement_markdown"`
	DifficultyID        string                           `json:"difficulty_id"`
	TagIDs              []string                         `json:"tag_ids"`
	CPUTimeMs           int                              `json:"cpu_time_ms"`
	WallTimeMs          int                              `json:"wall_time_ms"`
	MemoryLimitKb       int                              `json:"memory_limit_kb"`
	OutputLimitBytes    int                              `json:"output_limit_bytes"`
	ProcessLimit        int                              `json:"process_limit"`
	CheckerKey          string                           `json:"checker_key"`
	CheckerVersion      string                           `json:"checker_version"`
	CheckerConfig       json.RawMessage                  `json:"checker_config"`
	AllowedRuntimeKeys  []string                         `json:"allowed_runtime_keys"`
	ArtifactStatus      string                           `json:"artifact_status"`
	ArtifactDiagnostics []string                         `json:"artifact_diagnostics"`
	SavedAt             time.Time                        `json:"saved_at"`
	Groups              []AuthoringTestcaseGroupResponse `json:"groups"`
}

type ProblemAuthoringCatalogResponse struct {
	ReleaseID      string                               `json:"release_id"`
	CatalogVersion string                               `json:"catalog_version"`
	Runtimes       []ProblemAuthoringRuntimeResponse    `json:"runtimes"`
	Checkers       []ProblemAuthoringCheckerResponse    `json:"checkers"`
	Capabilities   []ProblemAuthoringCapabilityResponse `json:"capabilities"`
	Policy         ProblemAuthoringPolicyResponse       `json:"policy"`
}

type ProblemAuthoringRuntimeResponse struct {
	Key            string `json:"key"`
	Language       string `json:"language"`
	DisplayName    string `json:"display_name"`
	ProfileVersion string `json:"profile_version"`
}

type ProblemAuthoringCheckerResponse struct {
	Key        string `json:"key"`
	Version    string `json:"version"`
	ConfigKind string `json:"config_kind"`
}

type ProblemAuthoringCapabilityResponse struct {
	RuntimeKey     string `json:"runtime_key"`
	CheckerKey     string `json:"checker_key"`
	CheckerVersion string `json:"checker_version"`
}

type ProblemAuthoringNumericPolicyResponse struct {
	Default int `json:"default"`
	Minimum int `json:"minimum"`
	Maximum int `json:"maximum"`
}

type ProblemAuthoringPolicyResponse struct {
	SlugMaximumBytes      int                                   `json:"slug_maximum_bytes"`
	TitleMaximumBytes     int                                   `json:"title_maximum_bytes"`
	StatementMaximumBytes int                                   `json:"statement_maximum_bytes"`
	ReasonMaximumBytes    int                                   `json:"reason_maximum_bytes"`
	TestAssetMaximumBytes int                                   `json:"test_asset_maximum_bytes"`
	MaximumTestcases      int                                   `json:"maximum_testcases"`
	MaximumGroups         int                                   `json:"maximum_groups"`
	MaximumTags           int                                   `json:"maximum_tags"`
	MaximumRuntimes       int                                   `json:"maximum_runtimes"`
	CPUTimeMilliseconds   ProblemAuthoringNumericPolicyResponse `json:"cpu_time_ms"`
	WallTimeMilliseconds  ProblemAuthoringNumericPolicyResponse `json:"wall_time_ms"`
	MemoryKilobytes       ProblemAuthoringNumericPolicyResponse `json:"memory_limit_kb"`
	OutputBytes           ProblemAuthoringNumericPolicyResponse `json:"output_limit_bytes"`
	ProcessCount          ProblemAuthoringNumericPolicyResponse `json:"process_limit"`
}

type AuthoringTestcaseGroupResponse struct {
	StopOnFailure bool                        `json:"stop_on_failure"`
	Cases         []AuthoringTestcaseResponse `json:"cases"`
}

type AuthoringTestcaseResponse struct {
	TestCaseID     string `json:"test_case_id"`
	Kind           string `json:"kind"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
}

type ProblemPublicationResponse struct {
	ProblemID         string     `json:"problem_id"`
	RevisionID        *string    `json:"revision_id,omitempty"`
	TestsetRevisionID *string    `json:"testset_revision_id,omitempty"`
	Version           int64      `json:"version"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	Lifecycle         string     `json:"lifecycle"`
}

func stringIDs(ids []uuid.UUID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = id.String()
	}
	return result
}
