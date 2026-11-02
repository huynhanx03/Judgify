package usecase

import (
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
	entity "github.com/huynhanx03/judgify/internal/submission/domain"
)

// ToSubmissionResponse converts Submission entity to response DTO.
func ToSubmissionResponse(e *entity.Submission) *contract.SubmissionResponse {
	if e == nil {
		return nil
	}
	return &contract.SubmissionResponse{
		ID:           e.ID,
		ProblemID:    e.ProblemID,
		UserID:       e.UserID,
		Language:     e.Language,
		SourceCode:   e.SourceCode,
		Status:       e.Status,
		PassedCount:  e.PassedCount,
		TotalCount:   e.TotalCount,
		TimeMs:       e.TimeMs,
		MemoryKb:     e.MemoryKb,
		ContestID:    e.ContestID,
		ErrorMessage: e.ErrorMessage,
		CreatedAt:    e.CreatedAt,
	}
}

// ToJudgeOperationResponse maps the narrowly scoped durable operation result
// to a browser-safe acknowledgement.
func ToJudgeOperationResponse(result ports.OperationResult) *contract.JudgeOperationResponse {
	return &contract.JudgeOperationResponse{
		SubmissionID:     result.SubmissionID,
		Generation:       result.Generation,
		AggregateVersion: result.AggregateVersion,
		Phase:            string(result.Phase),
		Outcome:          result.Outcome,
		JobID:            result.JobID,
		IdempotentReplay: result.IdempotentReplay,
	}
}

// ToSubmissionSummaryResponse converts an entity to its bounded list
// projection. It intentionally cannot carry source code or judge diagnostics.
func ToSubmissionSummaryResponse(e *entity.Submission) *contract.SubmissionSummaryResponse {
	if e == nil {
		return nil
	}
	return &contract.SubmissionSummaryResponse{
		ID:          e.ID,
		ProblemID:   e.ProblemID,
		UserID:      e.UserID,
		ContestID:   e.ContestID,
		Language:    e.Language,
		Status:      e.Status,
		PassedCount: e.PassedCount,
		TotalCount:  e.TotalCount,
		TimeMs:      e.TimeMs,
		MemoryKb:    e.MemoryKb,
		CreatedAt:   e.CreatedAt,
	}
}

// ToAdminSubmissionSummaryResponse projects only operational metadata needed
// by the bounded monitoring list.
func ToAdminSubmissionSummaryResponse(e *entity.Submission) *contract.AdminSubmissionSummaryResponse {
	if e == nil {
		return nil
	}
	return &contract.AdminSubmissionSummaryResponse{
		ID:                        e.ID,
		ProblemID:                 e.ProblemID,
		UserID:                    e.UserID,
		ContestID:                 e.ContestID,
		Language:                  e.Language,
		Status:                    e.Status,
		PassedCount:               e.PassedCount,
		TotalCount:                e.TotalCount,
		TimeMs:                    e.TimeMs,
		MemoryKb:                  e.MemoryKb,
		LatestGenerationStartedAt: e.LatestGenerationStartedAt,
		JudgedAt:                  e.JudgedAt,
		VerdictVersion:            e.VerdictVersion,
		CreatedAt:                 e.CreatedAt,
		UpdatedAt:                 e.UpdatedAt,
	}
}

// ToAdminSubmissionDetailResponse adds source and the already-bounded judge
// diagnostic to the safe operational projection.
func ToAdminSubmissionDetailResponse(e *entity.Submission) *contract.AdminSubmissionDetailResponse {
	if e == nil {
		return nil
	}
	return &contract.AdminSubmissionDetailResponse{
		AdminSubmissionSummaryResponse: *ToAdminSubmissionSummaryResponse(e),
		SourceCode:                     e.SourceCode,
		ErrorMessage:                   e.ErrorMessage,
		AggregateVersion:               e.Version,
		LatestGeneration:               e.LatestGeneration,
		LatestGenerationPhase:          string(e.LatestGenerationPhase),
		LatestGenerationOutcome:        optionalGenerationOutcome(e.LatestGenerationOutcome),
	}
}

func optionalGenerationOutcome(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
