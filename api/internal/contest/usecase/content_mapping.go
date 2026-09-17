package usecase

import (
	"github.com/google/uuid"

	contract "github.com/huynhanx03/judgify/internal/contest/contract"
	entity "github.com/huynhanx03/judgify/internal/contest/domain"
	"github.com/huynhanx03/judgify/internal/ports"
)

func contentQuery(
	request *contract.ContestContentListRequest,
	actorID uuid.UUID,
) (ports.ContentListQuery, error) {
	if request == nil || request.ContestID == uuid.Nil {
		return ports.ContentListQuery{},
			ports.ErrInvalidContestContentCommand
	}
	cursor, err := ports.DecodeContentCursor(request.Cursor)
	if err != nil {
		return ports.ContentListQuery{}, err
	}
	limit := request.Limit
	if limit == 0 {
		limit = entity.DefaultContestContentPageSize
	}
	if limit < 1 || limit > entity.MaximumContestContentPageSize {
		return ports.ContentListQuery{},
			ports.ErrInvalidContestContentCommand
	}
	return ports.ContentListQuery{
		ContestID: request.ContestID,
		ActorID:   actorID,
		After:     cursor,
		Limit:     limit,
		Status:    request.Status,
	}, nil
}

func encodeNext(cursor *ports.ContentCursor) (*string, error) {
	if cursor == nil {
		return nil, nil
	}
	encoded, err := ports.EncodeContentCursor(*cursor)
	if err != nil {
		return nil, err
	}
	return &encoded, nil
}

func announcementPublicDTO(
	value ports.AnnouncementView,
) *contract.AnnouncementResponse {
	return &contract.AnnouncementResponse{
		ID:            value.Announcement.ID,
		ContestID:     value.Announcement.ContestID,
		Status:        string(value.Announcement.Status),
		Audience:      string(value.Announcement.Audience),
		Title:         dereferenceString(value.Revision.Title),
		SanitizedHTML: value.SanitizedHTML,
		Revision:      value.Revision.RevisionNumber,
		Version:       value.Announcement.Version,
		ScheduledFor:  value.Announcement.ScheduledFor,
		PublishedAt:   value.Announcement.PublishedAt,
		WithdrawnAt:   value.Announcement.WithdrawnAt,
		CreatedAt:     value.Announcement.CreatedAt,
		UpdatedAt:     value.Announcement.UpdatedAt,
	}
}

func announcementAdminDTO(
	value ports.AnnouncementView,
) *contract.AnnouncementResponse {
	response := announcementPublicDTO(value)
	markdown := value.Revision.Markdown
	response.Markdown = &markdown
	return response
}

func clarificationOwnDTO(
	value ports.ClarificationView,
) *contract.ClarificationResponse {
	response := clarificationBaseDTO(value)
	if value.Question != nil {
		questionMarkdown := value.Question.Markdown
		response.QuestionMarkdown = &questionMarkdown
	}
	response.QuestionHTML = value.QuestionHTML
	response.PublicSummaryHTML = value.PublicSummaryHTML
	response.AnswerHTML = value.AnswerHTML
	return response
}

func clarificationAdminDTO(
	value ports.ClarificationView,
) *contract.ClarificationResponse {
	response := clarificationOwnDTO(value)
	requester := value.Clarification.RequesterID
	response.RequesterID = &requester
	return response
}

func clarificationBaseDTO(
	value ports.ClarificationView,
) *contract.ClarificationResponse {
	return &contract.ClarificationResponse{
		ID:               value.Clarification.ID,
		ContestID:        value.Clarification.ContestID,
		ContestProblemID: value.Clarification.ContestProblemID,
		Status:           string(value.Clarification.Status),
		Audience:         string(value.Clarification.AnswerAudience),
		Version:          value.Clarification.Version,
		CreatedAt:        value.Clarification.CreatedAt,
		AnsweredAt:       value.Clarification.AnsweredAt,
		ClosedAt:         value.Clarification.ClosedAt,
		WithdrawnAt:      value.Clarification.WithdrawnAt,
		UpdatedAt:        value.Clarification.UpdatedAt,
	}
}

func sharedClarificationDTO(
	value ports.SharedClarificationView,
) contract.ClarificationResponse {
	return contract.ClarificationResponse{
		ID:                value.ID,
		ContestID:         value.ContestID,
		ContestProblemID:  value.ContestProblemID,
		Status:            string(value.Status),
		Audience:          string(value.Audience),
		Version:           value.Version,
		PublicSummaryHTML: &value.PublicSummaryHTML,
		AnswerHTML:        &value.AnswerHTML,
		CreatedAt:         value.CreatedAt,
		AnsweredAt:        &value.AnsweredAt,
		ClosedAt:          value.ClosedAt,
		UpdatedAt:         value.UpdatedAt,
	}
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func safeAnnouncementPayload(value ports.AnnouncementView) map[string]any {
	return map[string]any{
		"contest_id":      value.Announcement.ContestID.String(),
		"announcement_id": value.Announcement.ID.String(),
		"version":         value.Announcement.Version,
		"audience":        string(value.Announcement.Audience),
		"revision":        value.Revision.RevisionNumber,
		"title":           dereferenceString(value.Revision.Title),
	}
}

func safeClarificationPayload(value ports.ClarificationView) map[string]any {
	payload := map[string]any{
		"contest_id":         value.Clarification.ContestID.String(),
		"clarification_id":   value.Clarification.ID.String(),
		"requester_id":       value.Clarification.RequesterID.String(),
		"version":            value.Clarification.Version,
		"audience":           string(value.Clarification.AnswerAudience),
		"contest_problem_id": optionalID(value.Clarification.ContestProblemID),
	}
	if value.Clarification.AnswerAudience ==
		entity.ClarificationAudienceParticipants {
		payload["shared"] = true
	}
	return payload
}
