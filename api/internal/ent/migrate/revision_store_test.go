package migration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"github.com/DATA-DOG/go-sqlmock"

	"github.com/huynhanx03/go-common/pkg/correlation"
)

func TestRevisionStoreInitializesOnlyHistoryMetadata(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	mock.ExpectExec(createRevisionTableSQL).WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("initialize revision store: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("revision initialization SQL: %v", err)
	}
}

func TestRevisionStoreChecksExistenceWithoutCreatingMetadata(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}
	mock.ExpectQuery(`SELECT to_regclass($1) IS NOT NULL`).
		WithArgs("public.atlas_schema_revisions").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	exists, err := store.Exists(context.Background())
	if err != nil {
		t.Fatalf("inspect revision store: %v", err)
	}
	if exists {
		t.Fatal("revision store unexpectedly exists")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("revision existence SQL: %v", err)
	}
}

func TestRevisionStoreRedactsFailurePayloadAndKeepsCID(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}
	revision := &atlasmigrate.Revision{
		Version:         "20260719040000",
		Description:     "identity_security",
		Type:            atlasmigrate.RevisionTypeExecute,
		Applied:         1,
		Total:           2,
		ExecutedAt:      time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC),
		ExecutionTime:   time.Millisecond,
		Error:           "database rejected secret value do-not-persist",
		ErrorStmt:       "INSERT INTO credentials(secret) VALUES ('do-not-persist')",
		Hash:            "file-hash",
		PartialHashes:   []string{"h1:statement-one"},
		OperatorVersion: "judgify/test",
	}
	ctx := correlation.WithContext(context.Background(), "migration-42")
	mock.ExpectExec(upsertRevisionSQL).
		WithArgs(
			revision.Version,
			revision.Description,
			int64(revision.Type),
			revision.Applied,
			revision.Total,
			revision.ExecutedAt,
			int64(revision.ExecutionTime),
			"migration execution failed; cid=migration-42",
			"statement 2 redacted",
			revision.Hash,
			[]byte(`["h1:statement-one"]`),
			revision.OperatorVersion,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.WriteRevision(ctx, revision); err != nil {
		t.Fatalf("write failed revision: %v", err)
	}
	if strings.Contains(revision.Error+revision.ErrorStmt, "redacted") {
		t.Fatal("revision supplied by Atlas was mutated while sanitizing storage")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redacted revision SQL: %v", err)
	}
}

func TestRevisionStoreRoundTripContract(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}

	executedAt := time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC)
	revision := &atlasmigrate.Revision{
		Version:         "20260719040000",
		Description:     "identity_security",
		Type:            atlasmigrate.RevisionTypeExecute,
		Applied:         2,
		Total:           3,
		ExecutedAt:      executedAt,
		ExecutionTime:   125 * time.Millisecond,
		Hash:            "h1:file-hash",
		PartialHashes:   []string{"h1:statement-one", "h1:statement-two"},
		OperatorVersion: "judgify/test",
	}
	mock.ExpectExec(upsertRevisionSQL).
		WithArgs(
			revision.Version,
			revision.Description,
			int64(revision.Type),
			revision.Applied,
			revision.Total,
			revision.ExecutedAt,
			int64(revision.ExecutionTime),
			"",
			"",
			revision.Hash,
			[]byte(`["h1:statement-one","h1:statement-two"]`),
			revision.OperatorVersion,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.WriteRevision(context.Background(), revision); err != nil {
		t.Fatalf("write revision: %v", err)
	}

	selectOne := `SELECT ` + selectRevisionColumns + `
FROM "public"."atlas_schema_revisions"
WHERE "version" = $1`
	mock.ExpectQuery(selectOne).
		WithArgs(revision.Version).
		WillReturnRows(sqlmock.NewRows(revisionColumnNames()).AddRow(
			revision.Version,
			revision.Description,
			int64(revision.Type),
			revision.Applied,
			revision.Total,
			revision.ExecutedAt,
			int64(revision.ExecutionTime),
			"",
			"",
			revision.Hash,
			[]byte(`["h1:statement-one","h1:statement-two"]`),
			revision.OperatorVersion,
		))
	loaded, err := store.ReadRevision(context.Background(), revision.Version)
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}
	if loaded.Version != revision.Version || loaded.Applied != revision.Applied ||
		len(loaded.PartialHashes) != 2 || loaded.PartialHashes[1] != "h1:statement-two" {
		t.Fatalf("loaded revision = %#v", loaded)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("revision round-trip SQL: %v", err)
	}
}

func TestRevisionStoreAcceptsAtlasLastStatementCheckpointBeforeFinalCleanup(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}
	revision := &atlasmigrate.Revision{
		Version:         "20260719040000",
		Description:     "identity_security",
		Type:            atlasmigrate.RevisionTypeExecute,
		Applied:         2,
		Total:           2,
		ExecutedAt:      time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC),
		ExecutionTime:   time.Millisecond,
		Hash:            "h1:file-hash",
		PartialHashes:   []string{"h1:statement-one", "h1:statement-two"},
		OperatorVersion: "judgify/test",
	}
	mock.ExpectExec(upsertRevisionSQL).
		WithArgs(
			revision.Version,
			revision.Description,
			int64(revision.Type),
			revision.Applied,
			revision.Total,
			revision.ExecutedAt,
			int64(revision.ExecutionTime),
			"",
			"",
			revision.Hash,
			[]byte(`["h1:statement-one","h1:statement-two"]`),
			revision.OperatorVersion,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.WriteRevision(context.Background(), revision); err != nil {
		t.Fatalf("write Atlas last-statement checkpoint: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("last-statement checkpoint SQL: %v", err)
	}
}

func TestValidateRevisionRejectsPartialHashProgressMismatch(t *testing.T) {
	t.Parallel()

	revision := &atlasmigrate.Revision{
		Version:         "20260719040000",
		Description:     "identity_security",
		Type:            atlasmigrate.RevisionTypeExecute,
		Applied:         1,
		Total:           2,
		ExecutedAt:      time.Date(2026, time.July, 19, 4, 0, 0, 0, time.UTC),
		Hash:            "h1:file-hash",
		OperatorVersion: "judgify/test",
	}
	if err := validateRevision(revision); err == nil ||
		!strings.Contains(err.Error(), "invalid partial hash progress") {
		t.Fatalf("mismatched partial hash progress error = %v", err)
	}
}

func TestRevisionStoreMapsMissingRowsToAtlasContract(t *testing.T) {
	t.Parallel()

	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store, err := newRevisionStore(database)
	if err != nil {
		t.Fatalf("new revision store: %v", err)
	}

	selectOne := `SELECT ` + selectRevisionColumns + `
FROM "public"."atlas_schema_revisions"
WHERE "version" = $1`
	mock.ExpectQuery(selectOne).
		WithArgs("20260719040000").
		WillReturnRows(sqlmock.NewRows(revisionColumnNames()))
	_, err = store.ReadRevision(context.Background(), "20260719040000")
	if !errors.Is(err, atlasmigrate.ErrRevisionNotExist) {
		t.Fatalf("missing revision error = %v, want Atlas ErrRevisionNotExist", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("missing revision SQL: %v", err)
	}
}

func revisionColumnNames() []string {
	return []string{
		"version",
		"description",
		"type",
		"applied",
		"total",
		"executed_at",
		"execution_time",
		"error",
		"error_stmt",
		"hash",
		"partial_hashes",
		"operator_version",
	}
}
