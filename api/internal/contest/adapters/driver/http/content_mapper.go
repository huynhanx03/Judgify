package http

import (
	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
)

func toAnnouncementResponse(value *contract.AnnouncementResponse) *announcementResponse {
	if value == nil {
		return nil
	}
	return &announcementResponse{ID: value.ID.String(), ContestID: value.ContestID.String(), Status: value.Status, Audience: value.Audience, Title: value.Title, SanitizedHTML: value.SanitizedHTML, Revision: value.Revision, Version: value.Version, ScheduledFor: value.ScheduledFor, PublishedAt: value.PublishedAt, WithdrawnAt: value.WithdrawnAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Markdown: value.Markdown}
}
func toAnnouncementPageResponse(value *contract.ContestContentPage[contract.AnnouncementResponse]) *announcementPageResponse {
	if value == nil {
		return nil
	}
	items := make([]*announcementResponse, len(value.Items))
	for i := range value.Items {
		items[i] = toAnnouncementResponse(&value.Items[i])
	}
	return &announcementPageResponse{Items: items, NextCursor: value.NextCursor}
}
func toContestUUIDPointer(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}
func toClarificationResponse(value *contract.ClarificationResponse) *clarificationResponse {
	if value == nil {
		return nil
	}
	return &clarificationResponse{ID: value.ID.String(), ContestID: value.ContestID.String(), ContestProblemID: toContestUUIDPointer(value.ContestProblemID), Status: value.Status, Audience: value.Audience, Version: value.Version, QuestionHTML: value.QuestionHTML, QuestionMarkdown: value.QuestionMarkdown, PublicSummaryHTML: value.PublicSummaryHTML, AnswerHTML: value.AnswerHTML, RequesterID: toContestUUIDPointer(value.RequesterID), CreatedAt: value.CreatedAt, AnsweredAt: value.AnsweredAt, ClosedAt: value.ClosedAt, WithdrawnAt: value.WithdrawnAt, UpdatedAt: value.UpdatedAt}
}
func toClarificationPageResponse(value *contract.ContestContentPage[contract.ClarificationResponse]) *clarificationPageResponse {
	if value == nil {
		return nil
	}
	items := make([]*clarificationResponse, len(value.Items))
	for i := range value.Items {
		items[i] = toClarificationResponse(&value.Items[i])
	}
	return &clarificationPageResponse{Items: items, NextCursor: value.NextCursor}
}
