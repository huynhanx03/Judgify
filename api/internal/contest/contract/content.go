package contract

import (
	"time"

	"github.com/google/uuid"
)

type ContestContentListRequest struct {
	ContestID uuid.UUID
	Cursor    string
	Limit     int
	Status    string
}

type ContestContentIDRequest struct {
	ContestID uuid.UUID
	ContentID uuid.UUID
}

type CreateAnnouncementRequest struct {
	ContestID    uuid.UUID
	CommandID    uuid.UUID
	Audience     string
	Title        string
	Markdown     string
	ScheduledFor *time.Time
	Reason       string
}

type EditAnnouncementRequest struct {
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
	Audience        string
	Title           string
	Markdown        string
	ScheduledFor    *time.Time
	Reason          string
}

type AnnouncementCommandRequest struct {
	ContestID       uuid.UUID
	AnnouncementID  uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
	Reason          string
}

type SubmitClarificationRequest struct {
	ContestID        uuid.UUID
	CommandID        uuid.UUID
	ContestProblemID *uuid.UUID
	QuestionMarkdown string
}

type AnswerClarificationRequest struct {
	ContestID             uuid.UUID
	ClarificationID       uuid.UUID
	CommandID             uuid.UUID
	ExpectedVersion       int64
	Audience              string
	PublicSummaryMarkdown *string
	AnswerMarkdown        string
	Reason                string
}

type ClarificationCommandRequest struct {
	ContestID       uuid.UUID
	ClarificationID uuid.UUID
	CommandID       uuid.UUID
	ExpectedVersion int64
	Reason          string
}

type ContestContentPage[T any] struct {
	Items      []T
	NextCursor *string
}

type AnnouncementResponse struct {
	ID            uuid.UUID
	ContestID     uuid.UUID
	Status        string
	Audience      string
	Title         string
	SanitizedHTML string
	Revision      int64
	Version       int64
	ScheduledFor  *time.Time
	PublishedAt   *time.Time
	WithdrawnAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Markdown      *string
}

type ClarificationResponse struct {
	ID                uuid.UUID
	ContestID         uuid.UUID
	ContestProblemID  *uuid.UUID
	Status            string
	Audience          string
	Version           int64
	QuestionHTML      *string
	QuestionMarkdown  *string
	PublicSummaryHTML *string
	AnswerHTML        *string
	RequesterID       *uuid.UUID
	CreatedAt         time.Time
	AnsweredAt        *time.Time
	ClosedAt          *time.Time
	WithdrawnAt       *time.Time
	UpdatedAt         time.Time
}

type ContentRevisionResponse struct {
	ID             uuid.UUID
	ContentKind    string
	RevisionNumber int64
	Title          *string
	Markdown       string
	AuthorID       uuid.UUID
	CreatedAt      time.Time
}
