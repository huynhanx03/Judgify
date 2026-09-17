package http

import (
	"fmt"

	"github.com/google/uuid"
	commondto "github.com/huynhanx03/go-common/pkg/dto"
	"github.com/huynhanx03/judgify/internal/ports"
	contract "github.com/huynhanx03/judgify/internal/submission/contract"
)

func toSubmissionResponse(value *contract.SubmissionResponse) *submissionResponse {
	if value == nil {
		return nil
	}
	return &submissionResponse{ID: value.ID.String(), ProblemID: value.ProblemID.String(), UserID: value.UserID.String(), ContestID: toSubmissionUUIDPointer(value.ContestID), Language: value.Language, SourceCode: value.SourceCode, Status: value.Status, PassedCount: value.PassedCount, TotalCount: value.TotalCount, TimeMs: value.TimeMs, MemoryKb: value.MemoryKb, ErrorMessage: value.ErrorMessage, CreatedAt: value.CreatedAt}
}

func toSubmissionUUIDPointer(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func toSubmissionSummaryResponse(value *contract.SubmissionSummaryResponse) *submissionSummaryResponse {
	if value == nil {
		return nil
	}
	return &submissionSummaryResponse{ID: value.ID.String(), ProblemID: value.ProblemID.String(), UserID: value.UserID.String(), ContestID: toSubmissionUUIDPointer(value.ContestID), Language: value.Language, Status: value.Status, PassedCount: value.PassedCount, TotalCount: value.TotalCount, TimeMs: value.TimeMs, MemoryKb: value.MemoryKb, CreatedAt: value.CreatedAt}
}
func toSubmissionSummaryResponses(values []*contract.SubmissionSummaryResponse) []*submissionSummaryResponse {
	result := make([]*submissionSummaryResponse, len(values))
	for i, value := range values {
		result[i] = toSubmissionSummaryResponse(value)
	}
	return result
}

func toMySubmissionsPageResponse(value *contract.MySubmissionsPageResponse) *mySubmissionsPageResponse {
	if value == nil {
		return nil
	}
	return &mySubmissionsPageResponse{Items: toSubmissionSummaryResponses(value.Items), NextCursor: value.NextCursor, HasMore: value.HasMore}
}

func toAdminSubmissionSummaryResponse(value *contract.AdminSubmissionSummaryResponse) *adminSubmissionSummaryResponse {
	if value == nil {
		return nil
	}
	return &adminSubmissionSummaryResponse{ID: value.ID.String(), ProblemID: value.ProblemID.String(), UserID: value.UserID.String(), ContestID: toSubmissionUUIDPointer(value.ContestID), Language: value.Language, Status: value.Status, PassedCount: value.PassedCount, TotalCount: value.TotalCount, TimeMs: value.TimeMs, MemoryKb: value.MemoryKb, LatestGenerationStartedAt: value.LatestGenerationStartedAt, JudgedAt: value.JudgedAt, VerdictVersion: value.VerdictVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func toAdminSubmissionDetailResponse(value *contract.AdminSubmissionDetailResponse) *adminSubmissionDetailResponse {
	if value == nil {
		return nil
	}
	return &adminSubmissionDetailResponse{adminSubmissionSummaryResponse: toAdminSubmissionSummaryResponse(&value.AdminSubmissionSummaryResponse), SourceCode: value.SourceCode, ErrorMessage: value.ErrorMessage, AggregateVersion: value.AggregateVersion, LatestGeneration: value.LatestGeneration, LatestGenerationPhase: value.LatestGenerationPhase, LatestGenerationOutcome: value.LatestGenerationOutcome}
}

func toAdminSubmissionPageResponse(value *commondto.Paginated[*contract.AdminSubmissionSummaryResponse]) *commondto.Paginated[*adminSubmissionSummaryResponse] {
	if value == nil {
		return nil
	}
	page := &commondto.Paginated[*adminSubmissionSummaryResponse]{Pagination: value.Pagination}
	if value.Records != nil {
		values := make([]*adminSubmissionSummaryResponse, len(*value.Records))
		for i, item := range *value.Records {
			values[i] = toAdminSubmissionSummaryResponse(item)
		}
		page.Records = &values
	}
	return page
}

func toSubmissionID(request *submissionIDRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse submission id: %w", err)
	}
	return id, nil
}

func toCreateSubmissionCommand(request *createSubmissionRequest) (*contract.CreateSubmissionCommand, error) {
	problemID, err := uuid.Parse(request.ProblemID)
	if err != nil {
		return nil, fmt.Errorf("parse submission problem id: %w", err)
	}
	var contestID, contestProblemID *uuid.UUID
	if request.ContestID != nil {
		value, err := uuid.Parse(*request.ContestID)
		if err != nil {
			return nil, fmt.Errorf("parse submission contest id: %w", err)
		}
		contestID = &value
	}
	if request.ContestProblemID != nil {
		value, err := uuid.Parse(*request.ContestProblemID)
		if err != nil {
			return nil, fmt.Errorf("parse submission contest problem id: %w", err)
		}
		contestProblemID = &value
	}
	return &contract.CreateSubmissionCommand{ProblemID: problemID, RuntimeKey: request.RuntimeKey, SourceCode: request.SourceCode, ContestID: contestID, ContestProblemID: contestProblemID}, nil
}

func toRejudgeCommand(request *rejudgeSubmissionRequest, submissionID uuid.UUID, key uuid.UUID) ports.RejudgeCommand {
	return ports.RejudgeCommand{SubmissionID: submissionID, ExpectedVersion: request.ExpectedVersion, Reason: request.Reason, IdempotencyKey: key}
}

func toCancelCommand(request *cancelSubmissionRequest, submissionID uuid.UUID) ports.CancelCommand {
	return ports.CancelCommand{SubmissionID: submissionID, Generation: request.Generation, ExpectedVersion: request.ExpectedVersion, Reason: request.Reason}
}

func toJudgeOperationResponse(result *contract.JudgeOperationResponse) *judgeOperationResponse {
	if result == nil {
		return nil
	}
	response := &judgeOperationResponse{SubmissionID: result.SubmissionID.String(), Generation: result.Generation, AggregateVersion: result.AggregateVersion, Phase: result.Phase, Outcome: result.Outcome, IdempotentReplay: result.IdempotentReplay}
	if result.JobID != nil {
		jobID := result.JobID.String()
		response.JobID = &jobID
	}
	return response
}

func toSubmissionAcceptedResponse(result *contract.SubmissionAcceptedResponse) *submissionAcceptedResponse {
	if result == nil {
		return nil
	}
	response := &submissionAcceptedResponse{ID: result.ID.String(), AggregateVersion: result.AggregateVersion, LatestGeneration: result.LatestGeneration, LatestGenerationPhase: result.LatestGenerationPhase, LatestGenerationOutcome: result.LatestGenerationOutcome, ActiveJudgementGeneration: result.ActiveJudgementGeneration, ActiveVerdict: result.ActiveVerdict, SubmittedAt: result.SubmittedAt, ContestSubmissionSequence: result.ContestSubmissionSequence, CID: result.CID, IdempotentReplay: result.IdempotentReplay}
	if result.ActiveJudgementID != nil {
		value := result.ActiveJudgementID.String()
		response.ActiveJudgementID = &value
	}
	return response
}

func toProblemID(request *problemSubmissionsRequest) (uuid.UUID, error) {
	id, err := uuid.Parse(request.ProblemID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse submission problem id: %w", err)
	}
	return id, nil
}
