package contentrender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contentrenderartifact"
	"github.com/huynhanx03/judgify/internal/ent/generate/contentrendersource"
	_ "github.com/huynhanx03/judgify/internal/ent/generate/runtime"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

// EntRepository stores immutable render identities and artifacts. It never
// opens a transaction: callers compose writes in the active UnitOfWork.
type EntRepository struct {
	client *generate.Client
	now    func() time.Time
}

// NewEntRepository constructs a repository with a read-only fallback client.
// Mutations still require a transaction-bound context.
func NewEntRepository(client *generate.Client) (*EntRepository, error) {
	if client == nil {
		return nil, ErrInvalidRepository
	}
	return &EntRepository{
		client: client,
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

var _ Repository = (*EntRepository)(nil)

func (repository *EntRepository) RegisterSource(ctx context.Context, source Source) (Source, error) {
	client, err := repository.writeClient(ctx)
	if err != nil {
		return Source{}, err
	}
	if err := source.Validate(); err != nil {
		return Source{}, err
	}
	createdAt, err := repository.createdAt(source.CreatedAt)
	if err != nil {
		return Source{}, err
	}
	createdBy := cloneUUID(source.CreatedBy)
	checksum := append([]byte(nil), source.Checksum[:]...)

	record, err := client.ContentRenderSource.Create().
		SetID(source.ID).
		SetOwnerRevisionID(source.OwnerRevisionID).
		SetSourceKind(contentrendersource.SourceKind(source.Kind)).
		SetSourceChecksum(checksum).
		SetNillableCreatedBy(createdBy).
		SetCreatedAt(createdAt).
		Save(ctx)
	if err != nil {
		return Source{}, mapSourceWriteError(err)
	}
	return sourceFromRecord(record)
}

func (repository *EntRepository) GetSource(ctx context.Context, sourceID uuid.UUID) (Source, error) {
	if sourceID == uuid.Nil {
		return Source{}, ErrInvalidSourceID
	}
	client, err := repository.readClient(ctx)
	if err != nil {
		return Source{}, err
	}
	record, err := client.ContentRenderSource.Get(ctx, sourceID)
	if err != nil {
		return Source{}, mapSourceReadError(err)
	}
	return sourceFromRecord(record)
}

func (repository *EntRepository) StoreArtifact(
	ctx context.Context,
	source Source,
	artifact Artifact,
) (Artifact, error) {
	client, err := repository.writeClient(ctx)
	if err != nil {
		return Artifact{}, err
	}
	if artifact.ID != uuid.Nil {
		return Artifact{}, ErrArtifactAlreadyStored
	}
	if err := source.Validate(); err != nil {
		return Artifact{}, err
	}
	if err := artifact.ValidateFor(source); err != nil {
		return Artifact{}, err
	}

	persistedSource, err := repository.getSource(ctx, client, source.ID)
	if err != nil {
		return Artifact{}, err
	}
	if !sameSourceIdentity(source, persistedSource) {
		return Artifact{}, ErrArtifactSourceMismatch
	}

	createdAt, err := repository.createdAt(artifact.CreatedAt)
	if err != nil {
		return Artifact{}, err
	}
	diagnostics, err := json.Marshal(artifact.Diagnostics)
	if err != nil {
		return Artifact{}, fmt.Errorf("%w: encode diagnostics", ErrCorruptPersistence)
	}

	builder := client.ContentRenderArtifact.Create().
		SetID(commonent.NewUUID()).
		SetRenderSourceID(source.ID).
		SetRenderSourceKind(contentrenderartifact.RenderSourceKind(source.Kind)).
		SetSourceChecksum(append([]byte(nil), source.Checksum[:]...)).
		SetRendererVersion(artifact.RendererVersion).
		SetSanitizerPolicyVersion(artifact.SanitizerPolicyVersion).
		SetStatus(contentrenderartifact.Status(artifact.Status)).
		SetDiagnostics(json.RawMessage(diagnostics)).
		SetNillableCreatedBy(cloneUUID(artifact.CreatedBy)).
		SetCreatedAt(createdAt)
	if artifact.Status == StatusReady {
		builder.
			SetSanitizedHTML(artifact.SanitizedHTML).
			SetArtifactChecksum(append([]byte(nil), artifact.ArtifactChecksum[:]...))
	}

	record, err := builder.Save(ctx)
	if err != nil {
		return Artifact{}, mapArtifactWriteError(err)
	}
	return artifactFromRecord(record)
}

func (repository *EntRepository) GetArtifact(ctx context.Context, artifactID uuid.UUID) (Artifact, error) {
	if artifactID == uuid.Nil {
		return Artifact{}, ErrInvalidArtifactID
	}
	client, err := repository.readClient(ctx)
	if err != nil {
		return Artifact{}, err
	}
	record, err := client.ContentRenderArtifact.Get(ctx, artifactID)
	if err != nil {
		return Artifact{}, mapArtifactReadError(err)
	}
	return artifactFromRecord(record)
}

func (repository *EntRepository) FindArtifact(
	ctx context.Context,
	sourceID uuid.UUID,
	rendererVersion string,
	sanitizerPolicyVersion string,
) (Artifact, error) {
	if sourceID == uuid.Nil {
		return Artifact{}, ErrInvalidSourceID
	}
	if !validPipelineVersion(rendererVersion) || !validPipelineVersion(sanitizerPolicyVersion) {
		return Artifact{}, errors.New("content render: invalid artifact pipeline identity")
	}
	client, err := repository.readClient(ctx)
	if err != nil {
		return Artifact{}, err
	}
	record, err := client.ContentRenderArtifact.Query().
		Where(
			contentrenderartifact.RenderSourceID(sourceID),
			contentrenderartifact.RendererVersion(rendererVersion),
			contentrenderartifact.SanitizerPolicyVersion(sanitizerPolicyVersion),
		).
		Only(ctx)
	if err != nil {
		return Artifact{}, mapArtifactReadError(err)
	}
	return artifactFromRecord(record)
}

func (repository *EntRepository) getSource(
	ctx context.Context,
	client *generate.Client,
	sourceID uuid.UUID,
) (Source, error) {
	record, err := client.ContentRenderSource.Get(ctx, sourceID)
	if err != nil {
		return Source{}, mapSourceReadError(err)
	}
	return sourceFromRecord(record)
}

func (repository *EntRepository) writeClient(ctx context.Context) (*generate.Client, error) {
	if repository == nil || repository.client == nil || repository.now == nil || ctx == nil {
		return nil, ErrInvalidRepository
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return platformtx.Client(ctx, nil)
}

func (repository *EntRepository) readClient(ctx context.Context) (*generate.Client, error) {
	if repository == nil || repository.client == nil || repository.now == nil || ctx == nil {
		return nil, ErrInvalidRepository
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return platformtx.Client(ctx, repository.client)
}

func (repository *EntRepository) createdAt(value time.Time) (time.Time, error) {
	if repository == nil || repository.now == nil {
		return time.Time{}, ErrInvalidRepository
	}
	if value.IsZero() {
		value = repository.now()
	}
	if value.IsZero() {
		return time.Time{}, ErrInvalidRepository
	}
	return value.UTC(), nil
}

func sourceFromRecord(record *generate.ContentRenderSource) (Source, error) {
	if record == nil || record.ID == uuid.Nil || record.OwnerRevisionID == uuid.Nil || record.CreatedAt.IsZero() {
		return Source{}, ErrCorruptPersistence
	}
	checksum, err := digestFromBytes(record.SourceChecksum)
	if err != nil {
		return Source{}, err
	}
	source := Source{
		ID:              record.ID,
		OwnerRevisionID: record.OwnerRevisionID,
		Kind:            SourceKind(record.SourceKind),
		Checksum:        checksum,
		CreatedBy:       cloneUUID(record.CreatedBy),
		CreatedAt:       record.CreatedAt.UTC(),
	}
	if err := source.Validate(); err != nil {
		return Source{}, errors.Join(ErrCorruptPersistence, err)
	}
	return source, nil
}

func artifactFromRecord(record *generate.ContentRenderArtifact) (Artifact, error) {
	if record == nil || record.ID == uuid.Nil || record.RenderSourceID == uuid.Nil || record.CreatedAt.IsZero() {
		return Artifact{}, ErrCorruptPersistence
	}
	sourceChecksum, err := digestFromBytes(record.SourceChecksum)
	if err != nil {
		return Artifact{}, err
	}
	sourceKind := SourceKind(record.RenderSourceKind)
	if !sourceKind.Valid() {
		return Artifact{}, ErrCorruptPersistence
	}
	diagnostics, err := decodeDiagnostics(record.Diagnostics)
	if err != nil {
		return Artifact{}, err
	}
	artifact := Artifact{
		ID:                     record.ID,
		RendererVersion:        record.RendererVersion,
		SanitizerPolicyVersion: record.SanitizerPolicyVersion,
		Status:                 string(record.Status),
		Diagnostics:            diagnostics,
		CreatedBy:              cloneUUID(record.CreatedBy),
		CreatedAt:              record.CreatedAt.UTC(),
	}
	if record.SanitizedHTML != nil {
		artifact.SanitizedHTML = *record.SanitizedHTML
	}
	if record.ArtifactChecksum != nil {
		artifact.ArtifactChecksum, err = digestFromBytes(*record.ArtifactChecksum)
		if err != nil {
			return Artifact{}, err
		}
	}
	if err := artifact.Validate(); err != nil {
		return Artifact{}, errors.Join(ErrCorruptPersistence, err)
	}
	if artifact.Status == StatusReady {
		request := RenderRequest{
			SourceID:       record.RenderSourceID,
			SourceKind:     sourceKind,
			SourceChecksum: sourceChecksum,
		}
		if checksumArtifact(
			request,
			artifact.RendererVersion,
			artifact.SanitizerPolicyVersion,
			[]byte(artifact.SanitizedHTML),
		) != artifact.ArtifactChecksum {
			return Artifact{}, ErrCorruptPersistence
		}
	}
	return artifact, nil
}

func decodeDiagnostics(encoded json.RawMessage) ([]Diagnostic, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var diagnostics []Diagnostic
	if err := decoder.Decode(&diagnostics); err != nil {
		return nil, errors.Join(ErrCorruptPersistence, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, ErrCorruptPersistence
	}
	return diagnostics, nil
}

func digestFromBytes(value []byte) ([32]byte, error) {
	if len(value) != 32 {
		return [32]byte{}, ErrCorruptPersistence
	}
	var digest [32]byte
	copy(digest[:], value)
	if digest == ([32]byte{}) {
		return [32]byte{}, ErrCorruptPersistence
	}
	return digest, nil
}

func sameSourceIdentity(first, second Source) bool {
	return first.ID == second.ID &&
		first.OwnerRevisionID == second.OwnerRevisionID &&
		first.Kind == second.Kind &&
		first.Checksum == second.Checksum
}

type sqlStateError interface {
	error
	SQLState() string
}

func sqlState(err error) string {
	var state sqlStateError
	if errors.As(err, &state) {
		return state.SQLState()
	}
	return ""
}

func mapSourceWriteError(err error) error {
	if sqlState(err) == "23505" {
		return errors.Join(ErrSourceAlreadyExists, err)
	}
	return fmt.Errorf("content render: register source: %w", err)
}

func mapArtifactWriteError(err error) error {
	switch sqlState(err) {
	case "23505":
		return errors.Join(ErrArtifactAlreadyExists, err)
	case "23503":
		return errors.Join(ErrSourceNotFound, err)
	default:
		return fmt.Errorf("content render: store artifact: %w", err)
	}
}

func mapSourceReadError(err error) error {
	if generate.IsNotFound(err) {
		return errors.Join(ErrSourceNotFound, err)
	}
	return fmt.Errorf("content render: read source: %w", err)
}

func mapArtifactReadError(err error) error {
	if generate.IsNotFound(err) {
		return errors.Join(ErrArtifactNotFound, err)
	}
	return fmt.Errorf("content render: read artifact: %w", err)
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
