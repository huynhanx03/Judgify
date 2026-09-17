package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"

	"github.com/huynhanx03/go-common/pkg/correlation"
)

const (
	revisionTableName   = "atlas_schema_revisions"
	revisionTableSchema = "public"
	maxRevisionRows     = maxMigrationFiles + 1
	maxRevisionText     = 1_024
	maxRevisionHash     = 128
)

const createRevisionTableSQL = `CREATE TABLE IF NOT EXISTS "public"."atlas_schema_revisions" (
  "version" character varying(255) NOT NULL,
  "description" character varying(255) NOT NULL DEFAULT '',
  "type" bigint NOT NULL,
  "applied" bigint NOT NULL DEFAULT 0,
  "total" bigint NOT NULL DEFAULT 0,
  "executed_at" timestamptz NOT NULL,
  "execution_time" bigint NOT NULL DEFAULT 0,
  "error" text NOT NULL DEFAULT '',
  "error_stmt" text NOT NULL DEFAULT '',
  "hash" character varying(255) NOT NULL DEFAULT '',
  "partial_hashes" jsonb NOT NULL DEFAULT '[]'::jsonb,
  "operator_version" character varying(255) NOT NULL DEFAULT '',
  PRIMARY KEY ("version"),
  CONSTRAINT "atlas_schema_revisions_version" CHECK ("version" ~ '^[0-9]{14}$'),
  CONSTRAINT "atlas_schema_revisions_type" CHECK ("type" IN (1, 2)),
  CONSTRAINT "atlas_schema_revisions_progress" CHECK ("applied" >= 0 AND "total" >= 0 AND "applied" <= "total"),
  CONSTRAINT "atlas_schema_revisions_statement_limit" CHECK ("total" <= 50000),
  CONSTRAINT "atlas_schema_revisions_execution_time" CHECK ("execution_time" >= 0),
  CONSTRAINT "atlas_schema_revisions_partial_hashes" CHECK (
    jsonb_typeof("partial_hashes") = 'array'
    AND (jsonb_array_length("partial_hashes") = "applied"
      OR ("applied" = "total" AND jsonb_array_length("partial_hashes") = 0))
  ),
  CONSTRAINT "atlas_schema_revisions_hash" CHECK (
    ("type" = 1 AND "applied" = 0 AND "total" = 0 AND "hash" = '')
    OR ("type" = 2 AND "hash" <> '')
  ),
  CONSTRAINT "atlas_schema_revisions_metadata" CHECK (
    "operator_version" <> ''
    AND length("error") <= 1024
    AND length("error_stmt") <= 1024
  )
)`

const upsertRevisionSQL = `INSERT INTO "public"."atlas_schema_revisions" (
  "version", "description", "type", "applied", "total", "executed_at",
  "execution_time", "error", "error_stmt", "hash", "partial_hashes", "operator_version"
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT ("version") DO UPDATE SET
  "description" = EXCLUDED."description",
  "type" = EXCLUDED."type",
  "applied" = EXCLUDED."applied",
  "total" = EXCLUDED."total",
  "executed_at" = EXCLUDED."executed_at",
  "execution_time" = EXCLUDED."execution_time",
  "error" = EXCLUDED."error",
  "error_stmt" = EXCLUDED."error_stmt",
  "hash" = EXCLUDED."hash",
  "partial_hashes" = EXCLUDED."partial_hashes",
  "operator_version" = EXCLUDED."operator_version"`

const selectRevisionColumns = `"version", "description", "type", "applied", "total", "executed_at",
  "execution_time", "error", "error_stmt", "hash", "partial_hashes", "operator_version"`

type revisionStore struct {
	database revisionDatabase
}

type revisionDatabase interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func newRevisionStore(database revisionDatabase) (*revisionStore, error) {
	if database == nil {
		return nil, ErrInvalidConfiguration
	}
	return &revisionStore{database: database}, nil
}

func (*revisionStore) Ident() *atlasmigrate.TableIdent {
	return &atlasmigrate.TableIdent{Name: revisionTableName, Schema: revisionTableSchema}
}

func (store *revisionStore) Exists(ctx context.Context) (bool, error) {
	if store == nil || store.database == nil || ctx == nil {
		return false, ErrInvalidConfiguration
	}
	var exists bool
	if err := store.database.QueryRowContext(
		ctx,
		`SELECT to_regclass($1) IS NOT NULL`,
		revisionTableSchema+"."+revisionTableName,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("migration: inspect revision history table: %w", err)
	}
	return exists, nil
}

// Init creates only Atlas' operational history table. Product tables are
// created exclusively by checked-in SQL; this runner never invokes Ent
// auto-migration or schema diff/apply at runtime.
func (store *revisionStore) Init(ctx context.Context) error {
	if store == nil || store.database == nil || ctx == nil {
		return ErrInvalidConfiguration
	}
	if _, err := store.database.ExecContext(ctx, createRevisionTableSQL); err != nil {
		return fmt.Errorf("migration: initialize revision history: %w", err)
	}
	return nil
}

func (store *revisionStore) ReadRevisions(ctx context.Context) ([]*atlasmigrate.Revision, error) {
	if store == nil || store.database == nil || ctx == nil {
		return nil, ErrInvalidConfiguration
	}
	rows, err := store.database.QueryContext(ctx, `SELECT `+selectRevisionColumns+`
FROM "public"."atlas_schema_revisions"
ORDER BY "version" ASC
LIMIT 1001`)
	if err != nil {
		return nil, fmt.Errorf("migration: read revision history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	revisions := make([]*atlasmigrate.Revision, 0)
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
		if len(revisions) >= maxRevisionRows {
			return nil, historyDrift("revision count exceeds %d", maxMigrationFiles)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migration: iterate revision history: %w", err)
	}
	return revisions, nil
}

func (store *revisionStore) ReadRevision(
	ctx context.Context,
	version string,
) (*atlasmigrate.Revision, error) {
	if store == nil || store.database == nil || ctx == nil || version == "" {
		return nil, ErrInvalidConfiguration
	}
	row := store.database.QueryRowContext(ctx, `SELECT `+selectRevisionColumns+`
FROM "public"."atlas_schema_revisions"
WHERE "version" = $1`, version)
	revision, err := scanRevision(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, atlasmigrate.ErrRevisionNotExist
	}
	if err != nil {
		return nil, err
	}
	return revision, nil
}

func (store *revisionStore) WriteRevision(ctx context.Context, revision *atlasmigrate.Revision) error {
	if store == nil || store.database == nil || ctx == nil {
		return ErrInvalidConfiguration
	}
	stored := sanitizedRevision(ctx, revision)
	if err := validateRevision(stored); err != nil {
		return err
	}
	hashes := stored.PartialHashes
	if hashes == nil {
		hashes = []string{}
	}
	partialHashes, err := json.Marshal(hashes)
	if err != nil {
		return fmt.Errorf("migration: encode partial revision hashes: %w", err)
	}
	if _, err := store.database.ExecContext(
		ctx,
		upsertRevisionSQL,
		stored.Version,
		stored.Description,
		int64(stored.Type),
		stored.Applied,
		stored.Total,
		stored.ExecutedAt,
		int64(stored.ExecutionTime),
		stored.Error,
		stored.ErrorStmt,
		stored.Hash,
		partialHashes,
		stored.OperatorVersion,
	); err != nil {
		return fmt.Errorf("migration: write revision %q: %w", revision.Version, err)
	}
	return nil
}

func (store *revisionStore) DeleteRevision(ctx context.Context, version string) error {
	if store == nil || store.database == nil || ctx == nil || version == "" {
		return ErrInvalidConfiguration
	}
	result, err := store.database.ExecContext(ctx, `DELETE FROM "public"."atlas_schema_revisions" WHERE "version" = $1`, version)
	if err != nil {
		return fmt.Errorf("migration: delete revision %q: %w", version, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("migration: inspect deleted revision %q: %w", version, err)
	}
	if rows == 0 {
		return atlasmigrate.ErrRevisionNotExist
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanRevision(row rowScanner) (*atlasmigrate.Revision, error) {
	var (
		revision       atlasmigrate.Revision
		revisionType   int64
		executionNanos int64
		partialHashes  []byte
	)
	if err := row.Scan(
		&revision.Version,
		&revision.Description,
		&revisionType,
		&revision.Applied,
		&revision.Total,
		&revision.ExecutedAt,
		&executionNanos,
		&revision.Error,
		&revision.ErrorStmt,
		&revision.Hash,
		&partialHashes,
		&revision.OperatorVersion,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("migration: scan revision history: %w", err)
	}
	if revisionType < 0 || uint64(revisionType) > uint64(math.MaxUint) {
		return nil, fmt.Errorf("migration: invalid revision type %d", revisionType)
	}
	if executionNanos < 0 {
		return nil, fmt.Errorf("migration: invalid revision execution time %d", executionNanos)
	}
	revision.Type = atlasmigrate.RevisionType(revisionType)
	revision.ExecutionTime = time.Duration(executionNanos)
	if len(partialHashes) > 0 && string(partialHashes) != "null" {
		if err := json.Unmarshal(partialHashes, &revision.PartialHashes); err != nil {
			return nil, fmt.Errorf("migration: decode revision partial hashes: %w", err)
		}
	}
	if err := validateRevision(&revision); err != nil {
		return nil, err
	}
	return &revision, nil
}

func validateRevision(revision *atlasmigrate.Revision) error {
	if revision == nil || revision.Version == "" || revision.OperatorVersion == "" {
		return ErrInvalidConfiguration
	}
	if err := atlasmigrate.CheckVersion(revision.Version); err != nil {
		return fmt.Errorf("migration: revision %q has invalid version: %w", revision.Version, err)
	}
	if revision.Type != atlasmigrate.RevisionTypeBaseline && revision.Type != atlasmigrate.RevisionTypeExecute {
		return fmt.Errorf("migration: revision %q has invalid type %s", revision.Version, revision.Type)
	}
	if len(revision.Version) > 255 || len(revision.Description) > 255 ||
		len(revision.Hash) > maxRevisionHash || len(revision.OperatorVersion) > 255 ||
		len(revision.Error) > maxRevisionText || len(revision.ErrorStmt) > maxRevisionText {
		return fmt.Errorf("migration: revision %q contains oversized metadata", revision.Version)
	}
	for _, value := range []string{
		revision.Version,
		revision.Description,
		revision.Hash,
		revision.OperatorVersion,
		revision.Error,
		revision.ErrorStmt,
	} {
		if strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("migration: revision %q contains unsafe metadata", revision.Version)
		}
	}
	if revision.Applied < 0 || revision.Total < 0 || revision.Applied > revision.Total {
		return fmt.Errorf("migration: revision %q has invalid progress", revision.Version)
	}
	if revision.Total > maxMigrationStatements || len(revision.PartialHashes) > maxMigrationStatements {
		return fmt.Errorf("migration: revision %q exceeds statement limits", revision.Version)
	}
	if len(revision.PartialHashes) != revision.Applied &&
		(revision.Applied != revision.Total || len(revision.PartialHashes) != 0) {
		return fmt.Errorf("migration: revision %q has invalid partial hash progress", revision.Version)
	}
	for _, hash := range revision.PartialHashes {
		if len(hash) == 0 || len(hash) > maxRevisionHash || strings.ContainsAny(hash, "\x00\r\n") {
			return fmt.Errorf("migration: revision %q has invalid partial hashes", revision.Version)
		}
	}
	if revision.ExecutionTime < 0 || revision.ExecutedAt.IsZero() {
		return fmt.Errorf("migration: revision %q has invalid execution metadata", revision.Version)
	}
	return nil
}

func sanitizedRevision(ctx context.Context, revision *atlasmigrate.Revision) *atlasmigrate.Revision {
	if revision == nil {
		return nil
	}
	stored := *revision
	stored.PartialHashes = append([]string(nil), revision.PartialHashes...)
	if stored.Error != "" {
		stored.Error = "migration execution failed"
		if cid := correlation.FromContext(ctx); cid != "" {
			stored.Error += "; cid=" + cid
		}
	}
	if stored.ErrorStmt != "" {
		stored.ErrorStmt = fmt.Sprintf("statement %d redacted", stored.Applied+1)
	}
	return &stored
}

var _ atlasmigrate.RevisionReadWriter = (*revisionStore)(nil)
