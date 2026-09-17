package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/platform/contentrender"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

var (
	ErrInvalidProblemAggregate     = errors.New("problem: invalid stable aggregate")
	ErrInvalidRenderSelection      = errors.New("problem: invalid render selection")
	ErrInvalidPublicationCandidate = errors.New("problem: invalid publication candidate")
	ErrProblemArchived             = errors.New("problem: archived")
)

type Lifecycle string

const (
	LifecycleDraft     Lifecycle = problemcontract.LifecycleDraft
	LifecyclePublished Lifecycle = problemcontract.LifecyclePublished
	LifecycleArchived  Lifecycle = problemcontract.LifecycleArchived
)

// RenderSelection is the exact five-column aggregate pointer moved by draft
// save, publish, or an audited same-source security rerender.
type RenderSelection struct {
	RevisionID     uuid.UUID
	SourceID       uuid.UUID
	SourceKind     string
	SourceChecksum [32]byte
	ArtifactID     uuid.UUID
}

func (selection RenderSelection) Validate() error {
	if selection.RevisionID == uuid.Nil ||
		selection.SourceID == uuid.Nil ||
		selection.SourceKind != problemcontract.RenderSourceKindProblemStatement ||
		selection.SourceChecksum == ([32]byte{}) ||
		selection.ArtifactID == uuid.Nil {
		return ErrInvalidRenderSelection
	}
	return nil
}

// ProblemAggregate contains only stable identity, lifecycle, exact selectors,
// and rebuildable counters. Authored/judge facts live in immutable revisions.
type ProblemAggregate struct {
	ID               uuid.UUID
	Slug             string
	AuthorID         uuid.UUID
	TagMappingID     uuid.UUID
	ElementMappingID uuid.UUID
	Lifecycle        Lifecycle
	Version          int64
	Draft            *RenderSelection
	Published        *RenderSelection
	SubmissionCount  int64
	AcceptedCount    int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	PublishedAt      *time.Time
	ArchivedAt       *time.Time
}

func (aggregate ProblemAggregate) Validate() error {
	if aggregate.ID == uuid.Nil || aggregate.AuthorID == uuid.Nil ||
		aggregate.TagMappingID == uuid.Nil || aggregate.ElementMappingID == uuid.Nil ||
		!ValidProblemSlug(aggregate.Slug) || aggregate.Version < 1 ||
		!problemcontract.IsLifecycle(string(aggregate.Lifecycle)) ||
		aggregate.SubmissionCount < 0 || aggregate.AcceptedCount < 0 ||
		aggregate.AcceptedCount > aggregate.SubmissionCount ||
		aggregate.CreatedAt.IsZero() || aggregate.UpdatedAt.IsZero() {
		return ErrInvalidProblemAggregate
	}
	if aggregate.Draft != nil {
		if err := aggregate.Draft.Validate(); err != nil {
			return errors.Join(ErrInvalidProblemAggregate, err)
		}
	}
	if aggregate.Published != nil {
		if err := aggregate.Published.Validate(); err != nil {
			return errors.Join(ErrInvalidProblemAggregate, err)
		}
	}
	switch aggregate.Lifecycle {
	case LifecycleDraft:
		if aggregate.Published != nil || aggregate.PublishedAt != nil || aggregate.ArchivedAt != nil {
			return ErrInvalidProblemAggregate
		}
	case LifecyclePublished:
		if aggregate.Published == nil || aggregate.PublishedAt == nil ||
			aggregate.PublishedAt.IsZero() || aggregate.ArchivedAt != nil {
			return ErrInvalidProblemAggregate
		}
	case LifecycleArchived:
		if aggregate.Published == nil || aggregate.PublishedAt == nil ||
			aggregate.PublishedAt.IsZero() || aggregate.ArchivedAt == nil ||
			aggregate.ArchivedAt.IsZero() {
			return ErrInvalidProblemAggregate
		}
	default:
		return ErrInvalidProblemAggregate
	}
	return nil
}

// PublicationCandidate is loaded under the aggregate row lock. It contains
// internal authored/test content and must never be serialized directly.
type PublicationCandidate struct {
	Aggregate ProblemAggregate
	Revision  Revision
	Testset   TestsetRevision
	Source    contentrender.Source
	Artifact  contentrender.Artifact
}

func (candidate PublicationCandidate) Validate() error {
	if err := candidate.Aggregate.Validate(); err != nil {
		return errors.Join(ErrInvalidPublicationCandidate, err)
	}
	if candidate.Aggregate.Lifecycle == LifecycleArchived {
		return errors.Join(ErrInvalidPublicationCandidate, ErrProblemArchived)
	}
	if candidate.Aggregate.Draft == nil ||
		candidate.Aggregate.Draft.RevisionID != candidate.Revision.ID ||
		candidate.Aggregate.Draft.SourceID != candidate.Source.ID ||
		candidate.Aggregate.Draft.SourceKind != string(candidate.Source.Kind) ||
		candidate.Aggregate.Draft.SourceChecksum != candidate.Source.Checksum ||
		candidate.Aggregate.Draft.ArtifactID != candidate.Artifact.ID {
		return ErrInvalidPublicationCandidate
	}
	if err := candidate.Revision.Validate(); err != nil {
		return errors.Join(ErrInvalidPublicationCandidate, err)
	}
	if err := candidate.Testset.ValidateStructure(); err != nil {
		return errors.Join(ErrInvalidPublicationCandidate, err)
	}
	if candidate.Revision.ProblemID != candidate.Aggregate.ID ||
		candidate.Testset.ProblemID != candidate.Aggregate.ID ||
		candidate.Revision.TestsetRevisionID != candidate.Testset.ID ||
		candidate.Source.OwnerRevisionID != candidate.Revision.ID ||
		candidate.Source.ID != candidate.Revision.RenderSourceID ||
		string(candidate.Source.Kind) != candidate.Revision.RenderSourceKind ||
		candidate.Source.Checksum != candidate.Revision.RenderSourceChecksum ||
		candidate.Artifact.ID == uuid.Nil ||
		candidate.Artifact.Status != contentrender.StatusReady {
		return ErrInvalidPublicationCandidate
	}
	if err := candidate.Artifact.ValidateFor(candidate.Source); err != nil {
		return errors.Join(ErrInvalidPublicationCandidate, err)
	}
	return nil
}

func (candidate PublicationCandidate) Selection() (RenderSelection, error) {
	if err := candidate.Validate(); err != nil {
		return RenderSelection{}, err
	}
	return RenderSelection{
		RevisionID:     candidate.Revision.ID,
		SourceID:       candidate.Source.ID,
		SourceKind:     string(candidate.Source.Kind),
		SourceChecksum: candidate.Source.Checksum,
		ArtifactID:     candidate.Artifact.ID,
	}, nil
}
