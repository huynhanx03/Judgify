package domain

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaximumAnnouncementTitleBytes     = 200
	MaximumAnnouncementMarkdownBytes  = 64 << 10
	MaximumClarificationQuestionBytes = 16 << 10
	MaximumClarificationSummaryBytes  = 16 << 10
	MaximumClarificationAnswerBytes   = 32 << 10
	MaximumContestContentReasonBytes  = 1024
	DefaultContestContentPageSize     = 30
	MaximumContestContentPageSize     = 100
)

var (
	ErrContestContentInvalid           = errors.New("contest content: invalid value")
	ErrContestContentNotFound          = errors.New("contest content: not found")
	ErrContestContentForbidden         = errors.New("contest content: forbidden")
	ErrContestContentVersionConflict   = errors.New("contest content: version conflict")
	ErrContestContentStateConflict     = errors.New("contest content: invalid state transition")
	ErrContestContentNotReady          = errors.New("contest content: render artifact is not ready")
	ErrContestContentSchedulePending   = errors.New("contest content: scheduled publication is not due")
	ErrContestContentRegistration      = errors.New("contest content: active registration required")
	ErrContestContentContestNotRunning = errors.New("contest content: contest is not running")
)

type ContentOwnerType string

const (
	ContentOwnerAnnouncement  ContentOwnerType = "announcement"
	ContentOwnerClarification ContentOwnerType = "clarification"
)

func (value ContentOwnerType) Valid() bool {
	return value == ContentOwnerAnnouncement || value == ContentOwnerClarification
}

type ContentKind string

const (
	ContestDescriptionKind         ContentKind = "description"
	AnnouncementKind               ContentKind = "announcement_body"
	ClarificationQuestionKind      ContentKind = "question"
	ClarificationPublicSummaryKind ContentKind = "public_question_summary"
	ClarificationAnswerKind        ContentKind = "answer"
)

func (value ContentKind) Valid() bool {
	switch value {
	case ContestDescriptionKind, AnnouncementKind, ClarificationQuestionKind,
		ClarificationPublicSummaryKind, ClarificationAnswerKind:
		return true
	default:
		return false
	}
}

type AnnouncementStatus string

const (
	AnnouncementDraft     AnnouncementStatus = "draft"
	AnnouncementPublished AnnouncementStatus = "published"
	AnnouncementWithdrawn AnnouncementStatus = "withdrawn"
)

func (value AnnouncementStatus) Valid() bool {
	return value == AnnouncementDraft || value == AnnouncementPublished || value == AnnouncementWithdrawn
}

type AnnouncementAudience string

const (
	AnnouncementAudiencePublic       AnnouncementAudience = "public"
	AnnouncementAudienceParticipants AnnouncementAudience = "participants"
	AnnouncementAudienceJury         AnnouncementAudience = "jury"
)

func (value AnnouncementAudience) Valid() bool {
	return value == AnnouncementAudiencePublic || value == AnnouncementAudienceParticipants || value == AnnouncementAudienceJury
}

type ClarificationStatus string

const (
	ClarificationOpen      ClarificationStatus = "open"
	ClarificationAnswered  ClarificationStatus = "answered"
	ClarificationClosed    ClarificationStatus = "closed"
	ClarificationWithdrawn ClarificationStatus = "withdrawn"
)

func (value ClarificationStatus) Valid() bool {
	switch value {
	case ClarificationOpen, ClarificationAnswered, ClarificationClosed, ClarificationWithdrawn:
		return true
	default:
		return false
	}
}

type ClarificationAudience string

const (
	ClarificationAudienceRequester    ClarificationAudience = "requester"
	ClarificationAudienceParticipants ClarificationAudience = "participants"
)

func (value ClarificationAudience) Valid() bool {
	return value == ClarificationAudienceRequester || value == ClarificationAudienceParticipants
}

// RenderPointer identifies one exact immutable source and sanitized artifact.
// It deliberately contains no Markdown bytes.
type RenderPointer struct {
	RevisionID       uuid.UUID
	ContentKind      ContentKind
	RenderSourceID   uuid.UUID
	RenderSourceKind string
	SourceChecksum   [32]byte
	RenderArtifactID uuid.UUID
}

func (pointer RenderPointer) Validate() error {
	if pointer.RevisionID == uuid.Nil || !pointer.ContentKind.Valid() ||
		pointer.RenderSourceID == uuid.Nil || pointer.RenderSourceKind == "" ||
		pointer.SourceChecksum == ([32]byte{}) || pointer.RenderArtifactID == uuid.Nil {
		return ErrContestContentInvalid
	}
	return nil
}

// ContentRevision is append-only raw authored content. Raw Markdown is only
// returned through requester-owned or explicitly privileged mappings.
type ContentRevision struct {
	ID               uuid.UUID
	OwnerType        ContentOwnerType
	OwnerID          uuid.UUID
	Kind             ContentKind
	RevisionNumber   int64
	Title            *string
	Markdown         string
	AuthorID         uuid.UUID
	RenderSourceID   uuid.UUID
	RenderSourceKind string
	SourceChecksum   [32]byte
	RenderArtifactID uuid.UUID
	CreatedAt        time.Time
}

func (revision ContentRevision) Pointer() RenderPointer {
	return RenderPointer{
		RevisionID:       revision.ID,
		ContentKind:      revision.Kind,
		RenderSourceID:   revision.RenderSourceID,
		RenderSourceKind: revision.RenderSourceKind,
		SourceChecksum:   revision.SourceChecksum,
		RenderArtifactID: revision.RenderArtifactID,
	}
}

func (revision ContentRevision) Validate() error {
	if revision.ID == uuid.Nil || !revision.OwnerType.Valid() || revision.OwnerID == uuid.Nil ||
		!revision.Kind.Valid() || revision.RevisionNumber < 1 || revision.AuthorID == uuid.Nil ||
		revision.CreatedAt.IsZero() || revision.Pointer().Validate() != nil ||
		!validAuthoredMarkdown(revision.Markdown, maximumMarkdownBytes(revision.Kind)) {
		return ErrContestContentInvalid
	}
	if revision.Kind == AnnouncementKind {
		if revision.Title == nil || !validPlainContentText(*revision.Title, MaximumAnnouncementTitleBytes, false) ||
			strings.ContainsAny(*revision.Title, "\r\n") {
			return ErrContestContentInvalid
		}
	} else if revision.Title != nil {
		return ErrContestContentInvalid
	}
	return nil
}

type Announcement struct {
	ID           uuid.UUID
	ContestID    uuid.UUID
	Status       AnnouncementStatus
	Audience     AnnouncementAudience
	CurrentDraft RenderPointer
	Published    *RenderPointer
	Version      int64
	ScheduledFor *time.Time
	PublishedAt  *time.Time
	WithdrawnAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (announcement Announcement) Validate() error {
	if announcement.ID == uuid.Nil || announcement.ContestID == uuid.Nil ||
		!announcement.Status.Valid() || !announcement.Audience.Valid() ||
		announcement.CurrentDraft.Validate() != nil || announcement.CurrentDraft.ContentKind != AnnouncementKind ||
		announcement.Version < 1 || announcement.CreatedAt.IsZero() || announcement.UpdatedAt.IsZero() {
		return ErrContestContentInvalid
	}
	if announcement.ScheduledFor != nil && announcement.ScheduledFor.IsZero() {
		return ErrContestContentInvalid
	}
	switch announcement.Status {
	case AnnouncementDraft:
		if announcement.PublishedAt != nil || announcement.WithdrawnAt != nil {
			return ErrContestContentInvalid
		}
	case AnnouncementPublished:
		if announcement.Published == nil || announcement.Published.Validate() != nil ||
			announcement.Published.ContentKind != AnnouncementKind || announcement.PublishedAt == nil ||
			announcement.WithdrawnAt != nil {
			return ErrContestContentInvalid
		}
	case AnnouncementWithdrawn:
		if announcement.WithdrawnAt == nil {
			return ErrContestContentInvalid
		}
	}
	return nil
}

type Clarification struct {
	ID               uuid.UUID
	ContestID        uuid.UUID
	RequesterID      uuid.UUID
	ContestProblemID *uuid.UUID
	Status           ClarificationStatus
	AnswerAudience   ClarificationAudience
	Question         RenderPointer
	PublicSummary    *RenderPointer
	Answer           *RenderPointer
	Version          int64
	CreatedAt        time.Time
	AnsweredAt       *time.Time
	ClosedAt         *time.Time
	WithdrawnAt      *time.Time
	UpdatedAt        time.Time
}

func (clarification Clarification) Validate() error {
	if clarification.ID == uuid.Nil || clarification.ContestID == uuid.Nil || clarification.RequesterID == uuid.Nil ||
		(clarification.ContestProblemID != nil && *clarification.ContestProblemID == uuid.Nil) ||
		!clarification.Status.Valid() || !clarification.AnswerAudience.Valid() ||
		clarification.Question.Validate() != nil || clarification.Question.ContentKind != ClarificationQuestionKind ||
		clarification.Version < 1 || clarification.CreatedAt.IsZero() || clarification.UpdatedAt.IsZero() {
		return ErrContestContentInvalid
	}
	if clarification.AnswerAudience == ClarificationAudienceParticipants {
		if clarification.PublicSummary == nil || clarification.PublicSummary.Validate() != nil ||
			clarification.PublicSummary.ContentKind != ClarificationPublicSummaryKind {
			return ErrContestContentInvalid
		}
	}
	if clarification.Answer != nil && (clarification.Answer.Validate() != nil || clarification.Answer.ContentKind != ClarificationAnswerKind) {
		return ErrContestContentInvalid
	}
	switch clarification.Status {
	case ClarificationOpen:
		if clarification.Answer != nil || clarification.AnsweredAt != nil || clarification.ClosedAt != nil || clarification.WithdrawnAt != nil {
			return ErrContestContentInvalid
		}
	case ClarificationAnswered:
		if clarification.Answer == nil || clarification.AnsweredAt == nil || clarification.ClosedAt != nil || clarification.WithdrawnAt != nil {
			return ErrContestContentInvalid
		}
	case ClarificationClosed:
		if clarification.ClosedAt == nil || clarification.WithdrawnAt != nil {
			return ErrContestContentInvalid
		}
	case ClarificationWithdrawn:
		if clarification.WithdrawnAt == nil || clarification.Answer != nil || clarification.AnsweredAt != nil || clarification.ClosedAt != nil {
			return ErrContestContentInvalid
		}
	}
	return nil
}

func ValidateContestContentReason(value string, required bool) error {
	if !validPlainContentText(value, MaximumContestContentReasonBytes, !required) {
		return ErrContestContentInvalid
	}
	return nil
}

func maximumMarkdownBytes(kind ContentKind) int {
	switch kind {
	case AnnouncementKind:
		return MaximumAnnouncementMarkdownBytes
	case ClarificationQuestionKind:
		return MaximumClarificationQuestionBytes
	case ClarificationPublicSummaryKind:
		return MaximumClarificationSummaryBytes
	case ClarificationAnswerKind:
		return MaximumClarificationAnswerBytes
	default:
		return 0
	}
}

func validAuthoredMarkdown(value string, maximumBytes int) bool {
	return maximumBytes > 0 && validPlainContentText(value, maximumBytes, false)
}

func validPlainContentText(value string, maximumBytes int, allowEmpty bool) bool {
	if maximumBytes < 1 || len(value) > maximumBytes || !utf8.ValidString(value) ||
		value != strings.TrimSpace(value) || (!allowEmpty && value == "") {
		return false
	}
	for _, character := range value {
		if character == '\n' || character == '\r' || character == '\t' {
			continue
		}
		if unicode.IsControl(character) || unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}
