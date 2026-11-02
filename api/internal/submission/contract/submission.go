package contract

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	commondto "github.com/huynhanx03/go-common/pkg/dto"

	"github.com/huynhanx03/judgify/pkg/judging"
)

var (
	ErrInvalidCreateSubmission = errors.New(
		"submission: invalid create request",
	)
	submissionRuntimeKeyPattern = regexp.MustCompile(
		`^[a-z0-9][a-z0-9._-]{0,63}$`,
	)
)

// CreateSubmissionCommand contains the data needed to submit code for judging.
type CreateSubmissionCommand struct {
	ProblemID        uuid.UUID
	RuntimeKey       string
	SourceCode       string
	ContestID        *uuid.UUID
	ContestProblemID *uuid.UUID
}

func ValidateCreateSubmissionCommand(command *CreateSubmissionCommand) error {
	if command == nil || command.ProblemID == uuid.Nil || (command.ContestID == nil) != (command.ContestProblemID == nil) || (command.ContestID != nil && (*command.ContestID == uuid.Nil || *command.ContestProblemID == uuid.Nil)) || !submissionRuntimeKeyPattern.MatchString(command.RuntimeKey) || command.RuntimeKey != strings.TrimSpace(command.RuntimeKey) {
		return ErrInvalidCreateSubmission
	}
	return judging.ValidateSourceCode(command.SourceCode)
}

// MySubmissionsRequest is a bounded keyset query. Cursor is an opaque
// service-owned token; clients must never derive database ordering fields.
type MySubmissionsRequest struct {
	ProblemID uuid.UUID
	ContestID *uuid.UUID
	Cursor    string
	Limit     int
}

// AdminSubmissionFindRequest is a closed operational query contract. Unlike
// general-purpose find DTOs it cannot select arbitrary fields or sort orders.
type AdminSubmissionFindRequest struct {
	Pagination *commondto.PaginationOptions
	Status     *string
	Language   *string
	ProblemID  *uuid.UUID
	UserID     *uuid.UUID
	ContestID  *uuid.UUID
}

// JudgeOperationResponse is the safe acknowledgement for administrative
// rejudge/cancel mutations. It deliberately excludes source, diagnostics,
// lease capabilities and immutable hidden-test material.
type JudgeOperationResponse struct {
	SubmissionID     uuid.UUID
	Generation       int64
	AggregateVersion int64
	Phase            string
	Outcome          string
	JobID            *uuid.UUID
	IdempotentReplay bool
}

// SubmissionResponse represents submission data in API response.
type SubmissionResponse struct {
	ID           uuid.UUID
	ProblemID    uuid.UUID
	UserID       uuid.UUID
	ContestID    *uuid.UUID
	Language     string
	SourceCode   string
	Status       string
	PassedCount  int
	TotalCount   int
	TimeMs       *int
	MemoryKb     *int
	ErrorMessage *string
	CreatedAt    time.Time
}

// SubmissionAcceptedResponse is the committed resource snapshot returned by
// asynchronous intake. It deliberately makes no claim that a worker started.
type SubmissionAcceptedResponse struct {
	ID                        uuid.UUID
	AggregateVersion          int64
	LatestGeneration          int64
	LatestGenerationPhase     string
	LatestGenerationOutcome   *string
	ActiveJudgementID         *uuid.UUID
	ActiveJudgementGeneration *int64
	ActiveVerdict             *string
	SubmittedAt               time.Time
	ContestSubmissionSequence *int64
	CID                       string
	IdempotentReplay          bool
}

// SubmissionSummaryResponse is the list projection. Source code and internal
// judge errors are deliberately available only from the owner detail route.
type SubmissionSummaryResponse struct {
	ID          uuid.UUID
	ProblemID   uuid.UUID
	UserID      uuid.UUID
	ContestID   *uuid.UUID
	Language    string
	Status      string
	PassedCount int
	TotalCount  int
	TimeMs      *int
	MemoryKb    *int
	CreatedAt   time.Time
}

// MySubmissionsPageResponse is the stable owner-history contract. An absent
// continuation is encoded as null while has_more remains explicit.
type MySubmissionsPageResponse struct {
	Items      []*SubmissionSummaryResponse
	NextCursor *string
	HasMore    bool
}

// AdminSubmissionSummaryResponse is the monitoring list projection. Source,
// diagnostics, bundle identifiers, lease tokens, and test data cannot appear.
type AdminSubmissionSummaryResponse struct {
	ID                        uuid.UUID
	ProblemID                 uuid.UUID
	UserID                    uuid.UUID
	ContestID                 *uuid.UUID
	Language                  string
	Status                    string
	PassedCount               int
	TotalCount                int
	TimeMs                    *int
	MemoryKb                  *int
	LatestGenerationStartedAt *time.Time
	JudgedAt                  *time.Time
	VerdictVersion            int64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// AdminSubmissionDetailResponse is fetched only after an explicit detail
// request. It includes contestant source and bounded judge diagnostics, but no
// hidden input, expected output, judge bundle, lease, or reward internals.
type AdminSubmissionDetailResponse struct {
	AdminSubmissionSummaryResponse
	SourceCode   string
	ErrorMessage *string
	// Fencing state is deliberately the only generation metadata on the
	// administrative detail projection. No lease, worker, test, bundle, or
	// reward data crosses this boundary.
	AggregateVersion        int64
	LatestGeneration        int64
	LatestGenerationPhase   string
	LatestGenerationOutcome *string
}
