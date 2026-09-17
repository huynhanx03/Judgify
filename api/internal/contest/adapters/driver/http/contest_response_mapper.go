package http

import d "github.com/huynhanx03/go-common/pkg/dto"
import contract "github.com/huynhanx03/judgify/internal/contest/contract"

func toContestResponse(v *contract.ContestResponse) *contestResponse {
	if v == nil {
		return nil
	}
	r := &contestResponse{ID: v.ID.String(), Version: v.Version, Title: v.Title, Description: v.Description, StartTime: v.StartTime, EndTime: v.EndTime, Status: v.Status, AuthorID: v.AuthorID.String(), RegistrationMode: v.RegistrationMode, RegistrationOpensAt: v.RegistrationOpensAt, RegistrationClosesAt: v.RegistrationClosesAt, MaxParticipants: v.MaxParticipants, ParticipantCount: v.ParticipantCount, ProblemCount: v.ProblemCount, ScoringPolicy: v.ScoringPolicy, ScoringPolicyVersion: v.ScoringPolicyVersion, StandingsProjectionVersion: v.StandingsProjectionVersion, FreezeStandingsAt: v.FreezeStandingsAt, Rated: v.Rated, RatingAlgorithmVersion: v.RatingAlgorithmVersion, IsRegistered: v.IsRegistered, PublishedAt: v.PublishedAt, StartedAt: v.StartedAt, EndedAt: v.EndedAt, CancelledAt: v.CancelledAt, RatingCompletedAt: v.RatingCompletedAt, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
	if v.Registration != nil {
		r.Registration = &contestRegistrationResponse{ID: v.Registration.ID.String(), Status: v.Registration.Status, Version: v.Registration.Version}
	}
	r.DraftProblems = toContestDraftProblems(v.DraftProblems)
	r.Problems = toContestProblems(v.Problems)
	return r
}
func toContestDraftProblems(v []contract.ContestDraftProblemResponse) []contestDraftProblemResponse {
	r := make([]contestDraftProblemResponse, len(v))
	for i, x := range v {
		r[i] = contestDraftProblemResponse{ProblemID: x.ProblemID.String(), ProblemSlug: x.ProblemSlug, ProblemTitle: x.ProblemTitle, DisplayOrder: x.DisplayOrder, Alias: x.Alias, Points: x.Points, VisibleBeforeStart: x.VisibleBeforeStart}
	}
	return r
}
func toContestProblems(v []contract.ContestProblemResponse) []contestProblemResponse {
	r := make([]contestProblemResponse, len(v))
	for i, x := range v {
		r[i] = contestProblemResponse{ContestProblemID: x.ContestProblemID.String(), ProblemID: x.ProblemID.String(), ProblemSlug: x.ProblemSlug, ProblemTitle: x.ProblemTitle, DisplayOrder: x.DisplayOrder, Alias: x.Alias, Points: x.Points, VisibleBeforeStart: x.VisibleBeforeStart}
	}
	return r
}
func toContestPage(v *d.Paginated[*contract.ContestResponse]) *d.Paginated[*contestResponse] {
	if v == nil {
		return nil
	}
	r := &d.Paginated[*contestResponse]{Pagination: v.Pagination}
	if v.Records != nil {
		records := make([]*contestResponse, len(*v.Records))
		for i, x := range *v.Records {
			records[i] = toContestResponse(x)
		}
		r.Records = &records
	}
	return r
}
func toContestProblemDetailResponse(v *contract.ContestProblemDetailResponse) *contestProblemDetailResponse {
	if v == nil {
		return nil
	}
	r := &contestProblemDetailResponse{ContestID: v.ContestID.String(), ContestProblemID: v.ContestProblemID.String(), ProblemID: v.ProblemID.String(), ProblemSlug: v.ProblemSlug, RevisionNumber: v.RevisionNumber, Title: v.Title, StatementMarkdown: v.StatementMarkdown, Difficulty: contestProblemDifficultyResponse{ID: v.Difficulty.ID.String(), Revision: v.Difficulty.Revision, Name: v.Difficulty.Name, Level: v.Difficulty.Level, Description: v.Difficulty.Description}, CPUTimeMs: v.CPUTimeMs, WallTimeMs: v.WallTimeMs, MemoryLimitKb: v.MemoryLimitKb, OutputLimitBytes: v.OutputLimitBytes, ProcessLimit: v.ProcessLimit, AllowedRuntimeKeys: append([]string(nil), v.AllowedRuntimeKeys...), SubmissionCount: v.SubmissionCount, AcceptedCount: v.AcceptedCount, AcceptanceRate: v.AcceptanceRate, DisplayOrder: v.DisplayOrder, Alias: v.Alias, Points: v.Points, CreatedAt: v.CreatedAt}
	r.Tags = make([]contestProblemTagResponse, len(v.Tags))
	for i, x := range v.Tags {
		r.Tags[i] = contestProblemTagResponse{ID: x.ID.String(), Revision: x.Revision, Name: x.Name}
	}
	r.Samples = make([]contestProblemSampleResponse, len(v.Samples))
	for i, x := range v.Samples {
		r.Samples[i] = contestProblemSampleResponse{Input: x.Input, ExpectedOutput: x.ExpectedOutput, OrderIndex: x.OrderIndex}
	}
	return r
}
