package http

import contract "github.com/huynhanx03/judgify/internal/contest/contract"

func toCreateAnnouncementRequest(value *createAnnouncementRequest) *contract.CreateAnnouncementRequest {
	if value == nil {
		return nil
	}
	return &contract.CreateAnnouncementRequest{ContestID: value.ContestID, CommandID: value.CommandID, Audience: value.Audience, Title: value.Title, Markdown: value.Markdown, ScheduledFor: value.ScheduledFor, Reason: value.Reason}
}
func toEditAnnouncementRequest(value *editAnnouncementRequest) *contract.EditAnnouncementRequest {
	if value == nil {
		return nil
	}
	return &contract.EditAnnouncementRequest{ContestID: value.ContestID, AnnouncementID: value.AnnouncementID, CommandID: value.CommandID, ExpectedVersion: value.ExpectedVersion, Audience: value.Audience, Title: value.Title, Markdown: value.Markdown, ScheduledFor: value.ScheduledFor, Reason: value.Reason}
}
func toAnnouncementCommandRequest(value *announcementCommandRequest) *contract.AnnouncementCommandRequest {
	if value == nil {
		return nil
	}
	return &contract.AnnouncementCommandRequest{ContestID: value.ContestID, AnnouncementID: value.AnnouncementID, CommandID: value.CommandID, ExpectedVersion: value.ExpectedVersion, Reason: value.Reason}
}
func toSubmitClarificationRequest(value *submitClarificationRequest) *contract.SubmitClarificationRequest {
	if value == nil {
		return nil
	}
	return &contract.SubmitClarificationRequest{ContestID: value.ContestID, CommandID: value.CommandID, ContestProblemID: value.ContestProblemID, QuestionMarkdown: value.QuestionMarkdown}
}
func toAnswerClarificationRequest(value *answerClarificationRequest) *contract.AnswerClarificationRequest {
	if value == nil {
		return nil
	}
	return &contract.AnswerClarificationRequest{ContestID: value.ContestID, ClarificationID: value.ClarificationID, CommandID: value.CommandID, ExpectedVersion: value.ExpectedVersion, Audience: value.Audience, PublicSummaryMarkdown: value.PublicSummaryMarkdown, AnswerMarkdown: value.AnswerMarkdown, Reason: value.Reason}
}
func toClarificationCommandRequest(value *clarificationCommandRequest) *contract.ClarificationCommandRequest {
	if value == nil {
		return nil
	}
	return &contract.ClarificationCommandRequest{ContestID: value.ContestID, ClarificationID: value.ClarificationID, CommandID: value.CommandID, ExpectedVersion: value.ExpectedVersion, Reason: value.Reason}
}
func toContestContentListRequest(value *contestContentListRequest) *contract.ContestContentListRequest {
	if value == nil {
		return nil
	}
	return &contract.ContestContentListRequest{ContestID: value.ContestID, Cursor: value.Cursor, Limit: value.Limit, Status: value.Status}
}
func toContentRevisionResponses(values []contract.ContentRevisionResponse) []contentRevisionResponse {
	result := make([]contentRevisionResponse, len(values))
	for i, value := range values {
		result[i] = contentRevisionResponse{ID: value.ID.String(), ContentKind: value.ContentKind, RevisionNumber: value.RevisionNumber, Title: value.Title, Markdown: value.Markdown, AuthorID: value.AuthorID.String(), CreatedAt: value.CreatedAt}
	}
	return result
}
