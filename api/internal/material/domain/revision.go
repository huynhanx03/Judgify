package domain

import (
	"crypto/sha256"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
)

const (
	MaximumMaterialTitleBytes       = 255
	MaximumMaterialDescriptionBytes = 1000
	MaximumMaterialChangeBytes      = 500
	MaximumMaterialSlugBytes        = 160
	MaximumMaterialTags             = 64
	MaximumMaterialSearchBytes      = 256
	MaximumMaterialPageSize         = 100
)

type MaterialRevision struct {
	ID                       uuid.UUID
	MaterialID               uuid.UUID
	RevisionNumber           int64
	Title                    string
	Description              string
	ContentMarkdown          []byte
	CategoryID               uuid.UUID
	DifficultyID             uuid.UUID
	EstimatedReadMinutes     int
	AuthorID                 uuid.UUID
	CreatedAt                time.Time
	ChangeSummary            string
	NormalizedSourceChecksum [sha256.Size]byte
	RenderSourceID           uuid.UUID
	RenderSourceKind         contentrender.SourceKind
	RenderArtifactID         uuid.UUID
	TagIDs                   []uuid.UUID
}

type MaterialRevisionSummary struct {
	ID                       uuid.UUID
	MaterialID               uuid.UUID
	RevisionNumber           int64
	Title                    string
	Description              string
	CategoryID               uuid.UUID
	DifficultyID             uuid.UUID
	EstimatedReadMinutes     int
	AuthorID                 uuid.UUID
	CreatedAt                time.Time
	ChangeSummary            string
	NormalizedSourceChecksum [sha256.Size]byte
	RenderSourceID           uuid.UUID
	RenderArtifactID         uuid.UUID
	TagIDs                   []uuid.UUID
}

func (revision MaterialRevision) Validate() error {
	if revision.ID == uuid.Nil || revision.MaterialID == uuid.Nil ||
		revision.RevisionNumber < 1 || revision.CategoryID == uuid.Nil ||
		revision.DifficultyID == uuid.Nil || revision.AuthorID == uuid.Nil ||
		revision.RenderSourceID == uuid.Nil || revision.RenderArtifactID == uuid.Nil ||
		revision.RenderSourceKind != contentrender.MaterialBody ||
		revision.NormalizedSourceChecksum == ([sha256.Size]byte{}) ||
		revision.CreatedAt.IsZero() || revision.EstimatedReadMinutes < 1 ||
		!boundedText(revision.Title, 1, MaximumMaterialTitleBytes) ||
		!boundedText(revision.Description, 1, MaximumMaterialDescriptionBytes) ||
		!boundedText(revision.ChangeSummary, 1, MaximumMaterialChangeBytes) ||
		len(revision.ContentMarkdown) == 0 ||
		len(revision.TagIDs) > MaximumMaterialTags {
		return ErrMaterialInvalidCommand
	}
	seen := make(map[uuid.UUID]struct{}, len(revision.TagIDs))
	for _, tagID := range revision.TagIDs {
		if tagID == uuid.Nil {
			return ErrMaterialInvalidCommand
		}
		if _, duplicate := seen[tagID]; duplicate {
			return ErrMaterialInvalidCommand
		}
		seen[tagID] = struct{}{}
	}
	return nil
}

type RenderSelection struct {
	RevisionID     uuid.UUID
	SourceID       uuid.UUID
	SourceKind     contentrender.SourceKind
	SourceChecksum [sha256.Size]byte
	ArtifactID     uuid.UUID
}

func (selection RenderSelection) Validate() error {
	if selection.RevisionID == uuid.Nil || selection.SourceID == uuid.Nil ||
		selection.ArtifactID == uuid.Nil ||
		selection.SourceKind != contentrender.MaterialBody ||
		selection.SourceChecksum == ([sha256.Size]byte{}) {
		return ErrMaterialInvalidCommand
	}
	return nil
}

type MaterialSlug struct {
	ID         uuid.UUID
	MaterialID uuid.UUID
	Slug       string
	State      string
	CreatedAt  time.Time
	RetiredAt  *time.Time
}

type MaterialTag struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type PublicMaterial struct {
	MaterialID              uuid.UUID
	Slug                    string
	Title                   string
	Description             string
	SanitizedHTML           string
	CategoryID              uuid.UUID
	CategoryName            string
	DifficultyID            uuid.UUID
	DifficultyName          string
	DifficultyLevel         int
	Tags                    []MaterialTag
	AuthorID                uuid.UUID
	AuthorName              string
	EstimatedReadMinutes    int
	PublishedAt             time.Time
	ViewCount               int64
	Version                 int64
	RevisionID              uuid.UUID
	PublishedRevisionID     *uuid.UUID
	PublishedRevisionNumber *int64
	RevisionNumber          int64
	RenderArtifactID        uuid.UUID
	RendererVersion         string
	SanitizerPolicyVersion  string
	Visibility              MaterialVisibility
	Status                  MaterialState
	ContentMarkdown         string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type SlugResolution struct {
	Material      PublicMaterial
	Redirect      bool
	CanonicalSlug string
}

func NormalizeSlug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	builder.Grow(len(value))
	hyphen := false
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z',
			character >= '0' && character <= '9':
			builder.WriteRune(character)
			hyphen = false
		case character == '-' || character == ' ' || character == '_':
			if builder.Len() > 0 && !hyphen {
				builder.WriteByte('-')
				hyphen = true
			}
		default:
			return "", ErrMaterialInvalidCommand
		}
	}
	normalized := strings.Trim(builder.String(), "-")
	if !boundedText(normalized, 1, MaximumMaterialSlugBytes) {
		return "", ErrMaterialInvalidCommand
	}
	return normalized, nil
}

func EstimatedReadMinutes(markdown []byte) int {
	words := len(strings.Fields(string(markdown)))
	if words == 0 {
		return 1
	}
	minutes := (words + 199) / 200
	if minutes > 24*60 {
		return 24 * 60
	}
	return minutes
}

func boundedText(value string, minimum, maximum int) bool {
	if !utf8.ValidString(value) || len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if (unicode.IsControl(character) &&
			character != '\n' && character != '\t') ||
			unicode.In(character, unicode.Cf) {
			return false
		}
	}
	return true
}

func RequireReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if !boundedText(reason, 3, MaximumMaterialChangeBytes) {
		return "", ErrMaterialInvalidCommand
	}
	return reason, nil
}

func ValidateRevisionInput(title, description string, markdown []byte, tags []uuid.UUID) error {
	if !boundedText(strings.TrimSpace(title), 1, MaximumMaterialTitleBytes) ||
		!boundedText(strings.TrimSpace(description), 1, MaximumMaterialDescriptionBytes) ||
		len(markdown) == 0 || len(tags) > MaximumMaterialTags {
		return ErrMaterialInvalidCommand
	}
	if !utf8.Valid(markdown) {
		return errors.Join(ErrMaterialInvalidCommand, contentrender.ErrInvalidUTF8)
	}
	return nil
}
