package http

import (
	"time"

	"github.com/google/uuid"
)

type createAnnouncementRequest struct {
	Audience     string     `json:"audience" validate:"required,oneof=public participants jury"`
	Title        string     `json:"title" validate:"required,max=200"`
	Markdown     string     `json:"markdown" validate:"required,max=65536"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	Reason       string     `json:"reason" validate:"required,max=1024"`
	ContestID    uuid.UUID
	CommandID    uuid.UUID
}
type editAnnouncementRequest struct {
	Audience        string     `json:"audience" validate:"required,oneof=public participants jury"`
	Title           string     `json:"title" validate:"required,max=200"`
	Markdown        string     `json:"markdown" validate:"required,max=65536"`
	ScheduledFor    *time.Time `json:"scheduled_for,omitempty"`
	Reason          string     `json:"reason" validate:"required,max=1024"`
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
}
type announcementCommandRequest struct {
	Reason          string `json:"reason" validate:"required,max=1024"`
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
}
type submitClarificationRequest struct {
	ContestProblemID *uuid.UUID `json:"contest_problem_id,omitempty"`
	QuestionMarkdown string     `json:"question_markdown" validate:"required,max=16384"`
	ContestID        uuid.UUID
	CommandID        uuid.UUID
}
type answerClarificationRequest struct {
	Audience              string  `json:"audience" validate:"required,oneof=requester participants"`
	PublicSummaryMarkdown *string `json:"public_summary_markdown,omitempty" validate:"omitempty,max=16384"`
	AnswerMarkdown        string  `json:"answer_markdown" validate:"required,max=32768"`
	Reason                string  `json:"reason" validate:"required,max=1024"`
	ContestID             uuid.UUID
	ClarificationID       uuid.UUID
	CommandID             uuid.UUID
	ExpectedVersion       int64
}
type clarificationCommandRequest struct {
	Reason          string `json:"reason" validate:"required,max=1024"`
	ContestID       uuid.UUID
	ClarificationID uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
}
type contestContentListRequest struct {
	ContestID uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
	Cursor    string    `form:"cursor" validate:"omitempty,max=128"`
	Limit     int       `form:"limit" validate:"omitempty,min=1,max=100"`
	Status    string    `form:"status" validate:"omitempty,oneof=draft published withdrawn open answered closed"`
}
type contestContentIDRequest struct {
	ContestID uuid.UUID `uri:"id,parser=encoding.TextUnmarshaler" validate:"required"`
	ContentID uuid.UUID `uri:"content_id,parser=encoding.TextUnmarshaler" validate:"required"`
}
type contentRevisionResponse struct {
	ID             string    `json:"id"`
	ContentKind    string    `json:"content_kind"`
	RevisionNumber int64     `json:"revision_number"`
	Title          *string   `json:"title,omitempty"`
	Markdown       string    `json:"markdown"`
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type announcementResponse struct {
	ID            string     `json:"id"`
	ContestID     string     `json:"contest_id"`
	Status        string     `json:"status"`
	Audience      string     `json:"audience"`
	Title         string     `json:"title"`
	SanitizedHTML string     `json:"sanitized_html"`
	Revision      int64      `json:"revision"`
	Version       int64      `json:"version"`
	ScheduledFor  *time.Time `json:"scheduled_for,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	WithdrawnAt   *time.Time `json:"withdrawn_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Markdown      *string    `json:"markdown,omitempty"`
}
type announcementPageResponse struct {
	Items      []*announcementResponse `json:"items"`
	NextCursor *string                 `json:"next_cursor,omitempty"`
}
type clarificationResponse struct {
	ID                string     `json:"id"`
	ContestID         string     `json:"contest_id"`
	ContestProblemID  *string    `json:"contest_problem_id,omitempty"`
	Status            string     `json:"status"`
	Audience          string     `json:"audience"`
	Version           int64      `json:"version"`
	QuestionHTML      *string    `json:"question_html,omitempty"`
	QuestionMarkdown  *string    `json:"question_markdown,omitempty"`
	PublicSummaryHTML *string    `json:"public_summary_html,omitempty"`
	AnswerHTML        *string    `json:"answer_html,omitempty"`
	RequesterID       *string    `json:"requester_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	AnsweredAt        *time.Time `json:"answered_at,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	WithdrawnAt       *time.Time `json:"withdrawn_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
type clarificationPageResponse struct {
	Items      []*clarificationResponse `json:"items"`
	NextCursor *string                  `json:"next_cursor,omitempty"`
}
