package contentrender

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrSourceAlreadyExists   = errors.New("content render: source already exists")
	ErrArtifactAlreadyExists = errors.New("content render: artifact already exists")
	ErrSourceNotFound        = errors.New("content render: source not found")
	ErrArtifactNotFound      = errors.New("content render: artifact not found")
	ErrInvalidArtifactID     = errors.New("content render: invalid artifact id")
	ErrInvalidRepository     = errors.New("content render: invalid repository")
	ErrArtifactAlreadyStored = errors.New("content render: artifact is already persisted")
	ErrCorruptPersistence    = errors.New("content render: corrupt persistence record")
)

// Repository is intentionally append-only: there are no update/delete
// methods, and StoreArtifact receives the exact source identity it must bind.
type Repository interface {
	RegisterSource(ctx context.Context, source Source) (Source, error)
	GetSource(ctx context.Context, sourceID uuid.UUID) (Source, error)
	StoreArtifact(ctx context.Context, source Source, artifact Artifact) (Artifact, error)
	GetArtifact(ctx context.Context, artifactID uuid.UUID) (Artifact, error)
	FindArtifact(
		ctx context.Context,
		sourceID uuid.UUID,
		rendererVersion string,
		sanitizerPolicyVersion string,
	) (Artifact, error)
}
