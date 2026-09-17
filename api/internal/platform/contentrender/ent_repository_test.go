package contentrender

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	commonent "github.com/huynhanx03/go-common/pkg/database/ent"

	"github.com/huynhanx03/judgify/internal/ent/generate"
	"github.com/huynhanx03/judgify/internal/ent/generate/contentrenderartifact"
	"github.com/huynhanx03/judgify/internal/ent/generate/contentrendersource"
	platformtx "github.com/huynhanx03/judgify/internal/platform/transaction"
)

func TestNewEntRepositoryRejectsNilClient(t *testing.T) {
	t.Parallel()

	_, err := NewEntRepository(nil)
	require.ErrorIs(t, err, ErrInvalidRepository)
}

func TestEntRepositoryWritesRequireCallerUnitOfWork(t *testing.T) {
	t.Parallel()

	client, _ := newContentRenderMockClient(t)
	repository, err := NewEntRepository(client)
	require.NoError(t, err)
	source := validRepositorySource()

	_, err = repository.RegisterSource(context.Background(), source)
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)

	artifact := validReadyArtifact(t, source)
	_, err = repository.StoreArtifact(context.Background(), source, artifact)
	require.ErrorIs(t, err, platformtx.ErrTransactionRequired)
}

func TestEntRepositoryRegistersAndStoresAnArtifactInOneCallerTransaction(t *testing.T) {
	client, mock := newContentRenderMockClient(t)
	repository, err := NewEntRepository(client)
	require.NoError(t, err)
	fixedNow := time.Date(2026, time.July, 26, 9, 30, 0, 0, time.UTC)
	repository.now = func() time.Time { return fixedNow }
	source := validRepositorySource()
	artifact := validReadyArtifact(t, source)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "content_render_sources"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "content_render_sources"`).
		WillReturnRows(sqlmock.NewRows(contentrendersource.Columns).AddRow(
			source.ID,
			source.OwnerRevisionID,
			string(source.Kind),
			source.Checksum[:],
			nil,
			fixedNow,
		))
	mock.ExpectExec(`INSERT INTO "content_render_artifacts"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var storedSource Source
	var storedArtifact Artifact
	err = platformtx.NewEntUnitOfWork(client).Within(context.Background(), func(txCtx context.Context) error {
		var registerErr error
		storedSource, registerErr = repository.RegisterSource(txCtx, source)
		if registerErr != nil {
			return registerErr
		}
		var storeErr error
		storedArtifact, storeErr = repository.StoreArtifact(txCtx, storedSource, artifact)
		return storeErr
	})
	require.NoError(t, err)
	require.Equal(t, fixedNow, storedSource.CreatedAt)
	require.NotEqual(t, uuid.Nil, storedArtifact.ID)
	require.Equal(t, fixedNow, storedArtifact.CreatedAt)
	require.Equal(t, artifact.SanitizedHTML, storedArtifact.SanitizedHTML)
	require.Equal(t, artifact.ArtifactChecksum, storedArtifact.ArtifactChecksum)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEntRepositoryReadsThroughItsFallbackClient(t *testing.T) {
	t.Parallel()

	client, mock := newContentRenderMockClient(t)
	repository, err := NewEntRepository(client)
	require.NoError(t, err)
	source := validRepositorySource()
	createdAt := time.Date(2026, time.July, 26, 9, 30, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT .* FROM "content_render_sources"`).
		WillReturnRows(sqlmock.NewRows(contentrendersource.Columns).AddRow(
			source.ID,
			source.OwnerRevisionID,
			string(source.Kind),
			source.Checksum[:],
			nil,
			createdAt,
		))

	got, err := repository.GetSource(context.Background(), source.ID)
	require.NoError(t, err)
	require.Equal(t, source.ID, got.ID)
	require.Equal(t, source.Checksum, got.Checksum)
	require.Equal(t, createdAt, got.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestArtifactRecordMappingFailsClosedOnCorruptPersistence(t *testing.T) {
	t.Parallel()

	source := validRepositorySource()
	artifact := validReadyArtifact(t, source)
	record := readyArtifactRecord(source, artifact)

	got, err := artifactFromRecord(record)
	require.NoError(t, err)
	require.Equal(t, artifact.SanitizedHTML, got.SanitizedHTML)
	require.Equal(t, artifact.ArtifactChecksum, got.ArtifactChecksum)

	tests := map[string]func(*generate.ContentRenderArtifact){
		"nil id": func(value *generate.ContentRenderArtifact) {
			value.ID = uuid.Nil
		},
		"zero source checksum": func(value *generate.ContentRenderArtifact) {
			value.SourceChecksum = make([]byte, 32)
		},
		"unknown source kind": func(value *generate.ContentRenderArtifact) {
			value.RenderSourceKind = "unknown"
		},
		"unknown source kind on rejected artifact": func(value *generate.ContentRenderArtifact) {
			value.RenderSourceKind = "unknown"
			value.Status = contentrenderartifact.StatusRejected
			value.SanitizedHTML = nil
			value.ArtifactChecksum = nil
			value.Diagnostics = json.RawMessage(`[{"code":"render_rejected"}]`)
		},
		"checksum mismatch": func(value *generate.ContentRenderArtifact) {
			(*value.ArtifactChecksum)[0] ^= 0xff
		},
		"unknown diagnostic field": func(value *generate.ContentRenderArtifact) {
			value.Status = contentrenderartifact.StatusRejected
			value.SanitizedHTML = nil
			value.ArtifactChecksum = nil
			value.Diagnostics = json.RawMessage(`[{"code":"render_rejected","source":"secret"}]`)
		},
	}
	for name, mutate := range tests {
		mutate := mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			corrupt := readyArtifactRecord(source, artifact)
			mutate(corrupt)
			_, mappingErr := artifactFromRecord(corrupt)
			require.ErrorIs(t, mappingErr, ErrCorruptPersistence)
		})
	}
}

func TestRepositoryErrorMappingUsesSQLStateWithoutParsingMessages(t *testing.T) {
	t.Parallel()

	unique := sqlStateTestError{state: "23505"}
	foreignKey := sqlStateTestError{state: "23503"}
	require.ErrorIs(t, mapSourceWriteError(unique), ErrSourceAlreadyExists)
	require.ErrorIs(t, mapArtifactWriteError(unique), ErrArtifactAlreadyExists)
	require.ErrorIs(t, mapArtifactWriteError(foreignKey), ErrSourceNotFound)

	cancelled := context.Canceled
	require.ErrorIs(t, mapSourceWriteError(cancelled), context.Canceled)
	require.ErrorIs(t, mapArtifactWriteError(cancelled), context.Canceled)
}

func TestGeneratedContentRenderClientsRejectMutableOperationsBeforeSQL(t *testing.T) {
	t.Parallel()

	client, _ := newContentRenderMockClient(t)
	_, err := client.ContentRenderSource.Update().Save(context.Background())
	require.ErrorIs(t, err, commonent.ErrAppendOnlyMutation)

	_, err = client.ContentRenderArtifact.Delete().Exec(context.Background())
	require.ErrorIs(t, err, commonent.ErrAppendOnlyMutation)
}

func validRepositorySource() Source {
	return Source{
		ID:              uuid.MustParse("01987654-3210-7abc-8def-0123456789ab"),
		OwnerRevisionID: uuid.MustParse("01987654-3210-7abc-8def-1123456789ab"),
		Kind:            ProblemStatement,
		Checksum:        sha256.Sum256([]byte("registered source")),
	}
}

func validReadyArtifact(t *testing.T, source Source) Artifact {
	t.Helper()
	request := RenderRequest{
		SourceID:           source.ID,
		SourceKind:         source.Kind,
		SourceChecksum:     source.Checksum,
		NormalizedMarkdown: []byte("registered source"),
	}
	renderer, err := NewRenderer()
	require.NoError(t, err)
	artifact, err := renderer.Render(context.Background(), request)
	require.NoError(t, err)
	return artifact
}

func readyArtifactRecord(source Source, artifact Artifact) *generate.ContentRenderArtifact {
	html := artifact.SanitizedHTML
	checksum := append([]byte(nil), artifact.ArtifactChecksum[:]...)
	return &generate.ContentRenderArtifact{
		ID:                     uuid.MustParse("01987654-3210-7abc-8def-2123456789ab"),
		RenderSourceID:         source.ID,
		RenderSourceKind:       contentrenderartifact.RenderSourceKind(source.Kind),
		SourceChecksum:         append([]byte(nil), source.Checksum[:]...),
		RendererVersion:        artifact.RendererVersion,
		SanitizerPolicyVersion: artifact.SanitizerPolicyVersion,
		SanitizedHTML:          &html,
		ArtifactChecksum:       &checksum,
		Status:                 contentrenderartifact.Status(artifact.Status),
		Diagnostics:            json.RawMessage(`[]`),
		CreatedAt:              time.Date(2026, time.July, 26, 9, 30, 0, 0, time.UTC),
	}
}

func newContentRenderMockClient(t *testing.T) (*generate.Client, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, database)
	client := generate.NewClient(generate.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

type sqlStateTestError struct{ state string }

func (err sqlStateTestError) Error() string    { return "opaque persistence error" }
func (err sqlStateTestError) SQLState() string { return err.state }

func TestDigestFromBytesOwnsItsInput(t *testing.T) {
	t.Parallel()

	input := make([]byte, 32)
	input[0] = 1
	digest, err := digestFromBytes(input)
	require.NoError(t, err)
	input[0] = 2
	require.Equal(t, byte(1), digest[0])

	_, err = digestFromBytes(make([]byte, 32))
	require.ErrorIs(t, err, ErrCorruptPersistence)
	_, err = digestFromBytes(make([]byte, 31))
	require.ErrorIs(t, err, ErrCorruptPersistence)
}
