package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"

	problemEntity "github.com/huynhanx03/judgify/internal/problem/domain"
)

type MaterialState string

const (
	MaterialDraft     MaterialState = "draft"
	MaterialInReview  MaterialState = "in_review"
	MaterialPublished MaterialState = "published"
	MaterialArchived  MaterialState = "archived"
)

type MaterialVisibility string

const (
	MaterialPublic        MaterialVisibility = "public"
	MaterialAuthenticated MaterialVisibility = "authenticated"
)

var (
	ErrMaterialNotFound         = errors.New("material not found")
	ErrMaterialConflict         = errors.New("material version conflict")
	ErrMaterialCommandConflict  = errors.New("material command identity conflicts with committed request")
	ErrMaterialInvalidCommand   = errors.New("invalid material command")
	ErrMaterialInvalidLifecycle = errors.New("invalid material lifecycle transition")
	ErrMaterialArtifactRejected = errors.New("material render artifact rejected")
	ErrMaterialSlugConflict     = errors.New("material slug is unavailable")
)

// Material represents a learning material article.
type Material struct {
	ID                  uuid.UUID
	Slug                string
	CurrentSlugID       uuid.UUID
	CurrentDraftID      uuid.UUID
	PublishedRevisionID *uuid.UUID
	PublishedArtifactID *uuid.UUID
	Title               string
	Description         string
	Content             string
	DifficultyID        uuid.UUID
	Difficulty          *problemEntity.ProblemDifficulty
	AuthorID            uuid.UUID
	CategoryID          uuid.UUID
	Category            *MaterialCategory
	Status              string
	Visibility          string
	GroupID             *int
	ViewCount           int64
	Version             int64
	PublishedAt         *time.Time
	ArchivedAt          *time.Time
	EstimatedReadTime   int
	Tags                []problemEntity.Tag
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (m Material) State() MaterialState { return MaterialState(m.Status) }

func (state MaterialState) Valid() bool {
	switch state {
	case MaterialDraft, MaterialInReview, MaterialPublished, MaterialArchived:
		return true
	default:
		return false
	}
}

func (visibility MaterialVisibility) Valid() bool {
	return visibility == MaterialPublic || visibility == MaterialAuthenticated
}
