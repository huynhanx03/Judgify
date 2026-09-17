package migration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
)

func TestEmbeddedMigrationDirectoryIsChecksummedAndParseable(t *testing.T) {
	t.Parallel()

	directory, err := loadEmbeddedDirectory(context.Background())
	if err != nil {
		t.Fatalf("load embedded migration directory: %v", err)
	}
	files, err := directory.Files()
	if err != nil {
		t.Fatalf("list embedded migrations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("embedded migration directory is empty")
	}
	if err := atlasmigrate.Validate(directory); err != nil {
		t.Fatalf("validate embedded atlas.sum: %v", err)
	}
}

func TestBaselineContainsCurrentRecoveryCredentialColumnsAndIndexes(t *testing.T) {
	t.Parallel()

	data, err := embeddedMigrations.ReadFile("migrations/20260722073038_baseline.sql")
	if err != nil {
		t.Fatalf("read embedded baseline: %v", err)
	}
	sql := canonicalPublicSchemaSQL(string(data))
	for _, required := range []string{
		`"purpose" character varying NOT NULL`,
		`CONSTRAINT "recovery_contact_purpose_closed"`,
		`CREATE INDEX "userrecoverycontact_lookup_hash"`,
		`"hash_key_id" character varying NOT NULL`,
		`("hash_key_id", "token_hash")`,
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("baseline is missing recovery invariant %q", required)
		}
	}
	if strings.Contains(sql, `CREATE UNIQUE INDEX "userrecoverycontact_lookup_hash"`) {
		t.Fatal("baseline still makes a recovery address globally unique")
	}
}

func TestBaselineContainsRecoveryContactCommandReceiptInvariants(t *testing.T) {
	t.Parallel()

	data, err := embeddedMigrations.ReadFile("migrations/20260722073038_baseline.sql")
	if err != nil {
		t.Fatalf("read embedded baseline: %v", err)
	}
	sql := canonicalPublicSchemaSQL(string(data))
	for _, required := range []string{
		`CREATE TABLE "public"."recovery_contact_command_receipts"`,
		`"idempotency_key" uuid NOT NULL`,
		`"kind" character varying NOT NULL`,
		`"request_fingerprint" bytea NOT NULL`,
		`"expires_at" timestamptz NOT NULL`,
		`"session_id" uuid NOT NULL`,
		`"user_id" uuid NOT NULL`,
		`FOREIGN KEY ("session_id") REFERENCES "public"."auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION`,
		`FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION`,
		`CONSTRAINT "recovery_contact_command_kind_closed"`,
		`CREATE UNIQUE INDEX "recoverycontactcommandreceipt_user_id_idempotency_key" ON "public"."recovery_contact_command_receipts" ("user_id", "idempotency_key")`,
		`CREATE INDEX "recoverycontactcommandreceipt_expires_at" ON "public"."recovery_contact_command_receipts" ("expires_at")`,
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("baseline is missing recovery command receipt invariant %q", required)
		}
	}

	const tableStart = `CREATE TABLE "public"."recovery_contact_command_receipts" (`
	start := strings.Index(sql, tableStart)
	if start == -1 {
		t.Fatal("baseline is missing recovery contact command receipt table")
	}
	end := strings.Index(sql[start:], "\n);")
	if end == -1 {
		t.Fatal("recovery contact command receipt table is not terminated")
	}
	table := sql[start : start+end]
	for _, forbidden := range []string{"email", "address", "ciphertext", "payload"} {
		if strings.Contains(table, forbidden) {
			t.Fatalf("recovery command receipt persists forbidden plaintext-bearing field %q", forbidden)
		}
	}
}

func TestBaselineKeepsSessionCommandTargetForRevokeReceipts(t *testing.T) {
	t.Parallel()

	data, err := embeddedMigrations.ReadFile("migrations/20260722073038_baseline.sql")
	if err != nil {
		t.Fatalf("read embedded baseline: %v", err)
	}
	sql := canonicalPublicSchemaSQL(string(data))
	required := `CONSTRAINT "auth_session_command_receipts_auth_sessions_targeted_commands" FOREIGN KEY ("target_session_id") REFERENCES "public"."auth_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION`
	if !strings.Contains(sql, required) {
		t.Fatalf("session command target foreign key must retain revoke receipt target: %s", required)
	}
}

func TestBaselineContainsContentRenderIdentityInvariants(t *testing.T) {
	t.Parallel()

	data, err := embeddedMigrations.ReadFile("migrations/20260722073038_baseline.sql")
	if err != nil {
		t.Fatalf("read embedded baseline: %v", err)
	}
	sql := canonicalPublicSchemaSQL(string(data))
	for _, required := range []string{
		`CREATE TABLE "public"."content_render_sources"`,
		`CONSTRAINT "content_render_source_checksum_length" CHECK (octet_length(source_checksum) = 32)`,
		`CONSTRAINT "content_render_source_nonzero_identity"`,
		`CONSTRAINT "content_render_source_nonzero_checksum"`,
		`CONSTRAINT "content_render_source_kind_closed"`,
		`ON "public"."content_render_sources" ("source_kind", "owner_revision_id")`,
		`ON "public"."content_render_sources" ("id", "source_kind", "source_checksum")`,
		`ON "public"."content_render_sources" ("id", "owner_revision_id", "source_kind", "source_checksum")`,
		`CREATE TABLE "public"."content_render_artifacts"`,
		`CONSTRAINT "content_render_artifacts_content_render_sources_artifacts" FOREIGN KEY ("render_source_id") REFERENCES "public"."content_render_sources" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION`,
		`CONSTRAINT "content_render_artifacts_render_source_identity_fkey" FOREIGN KEY ("render_source_id", "render_source_kind", "source_checksum") REFERENCES "public"."content_render_sources" ("id", "source_kind", "source_checksum") ON UPDATE NO ACTION ON DELETE NO ACTION`,
		`CONSTRAINT "content_render_artifact_diagnostics_bound"`,
		`CONSTRAINT "content_render_artifact_diagnostics_safe_codes"`,
		`CONSTRAINT "content_render_artifact_html_bound"`,
		`CONSTRAINT "content_render_artifact_nonzero_identity"`,
		`CONSTRAINT "content_render_artifact_nonzero_checksums"`,
		`CONSTRAINT "content_render_artifact_renderer_version_bound"`,
		`CONSTRAINT "content_render_artifact_ready_shape"`,
		`CONSTRAINT "content_render_artifact_rejected_shape"`,
		`CONSTRAINT "content_render_artifact_source_checksum_length" CHECK (octet_length(source_checksum) = 32)`,
		`CONSTRAINT "content_render_artifact_source_kind_closed"`,
		`CONSTRAINT "content_render_artifact_sanitizer_policy_version_bound"`,
		`CONSTRAINT "content_render_artifact_status_closed"`,
		`ON "public"."content_render_artifacts" ("id", "render_source_id", "render_source_kind", "source_checksum")`,
		`ON "public"."content_render_artifacts" ("render_source_id", "renderer_version", "sanitizer_policy_version")`,
		`ON "public"."content_render_artifacts" ("render_source_id", "render_source_kind", "source_checksum")`,
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("baseline is missing content-render invariant %q", required)
		}
	}

	artifactAt := strings.Index(sql, `CREATE TABLE "public"."content_render_artifacts"`)
	if artifactAt == -1 {
		t.Fatal("content_render_artifacts table is missing")
	}
	artifactEnd := strings.Index(sql[artifactAt:], "\n);")
	if artifactEnd == -1 {
		t.Fatal("content_render_artifacts table is not terminated")
	}
	artifactTable := sql[artifactAt : artifactAt+artifactEnd]
	if strings.Contains(artifactTable, "CASCADE") || strings.Contains(artifactTable, "ON UPDATE SET") {
		t.Fatalf("content_render_artifacts has a mutable or cascading foreign-key path: %s", artifactTable)
	}

	appendOnlyDDL, err := schemaextras.JudgifyAppendOnlyDDL("public")
	if err != nil {
		t.Fatalf("build canonical append-only DDL: %v", err)
	}
	if count := strings.Count(sql, appendOnlyDDL); count != 1 {
		t.Fatalf("baseline contains canonical append-only DDL %d times, want exactly once", count)
	}
}

func TestLoadDirectoryRejectsChecksumDriftBeforeDatabaseWork(t *testing.T) {
	t.Parallel()

	source := checkedMigrationFS(t, map[string]string{
		"20260719000000_initial.sql": "CREATE TABLE example (id bigint PRIMARY KEY);",
	})
	source["20260719000000_initial.sql"] = &fstest.MapFile{
		Data: []byte("CREATE TABLE tampered (id bigint PRIMARY KEY);"),
	}
	_, err := loadDirectory(context.Background(), source)
	if !errors.Is(err, atlasmigrate.ErrChecksumMismatch) {
		t.Fatalf("load tampered directory error = %v, want checksum mismatch", err)
	}
}

func TestLoadDirectoryRejectsDuplicateVersions(t *testing.T) {
	t.Parallel()

	source := checkedMigrationFS(t, map[string]string{
		"20260719000000_first.sql":  "CREATE TABLE first_table (id bigint PRIMARY KEY);",
		"20260719000000_second.sql": "CREATE TABLE second_table (id bigint PRIMARY KEY);",
	})
	_, err := loadDirectory(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "duplicate version") {
		t.Fatalf("load duplicate versions error = %v", err)
	}
}

func TestLoadDirectoryRejectsNonCanonicalSQLSuffixAndCheckpoints(t *testing.T) {
	t.Parallel()

	upper := checkedMigrationFS(t, map[string]string{
		"20260719000000_initial.SQL": "CREATE TABLE example (id bigint PRIMARY KEY);",
	})
	if _, err := loadDirectory(context.Background(), upper); err == nil ||
		!strings.Contains(err.Error(), "unsupported checked-in file") {
		t.Fatalf("uppercase SQL suffix error = %v", err)
	}

	checkpoint := checkedMigrationFS(t, map[string]string{
		"20260719000000_checkpoint.sql": "-- atlas:checkpoint initial\n\nCREATE TABLE example (id bigint PRIMARY KEY);",
	})
	if _, err := loadDirectory(context.Background(), checkpoint); err == nil ||
		!strings.Contains(err.Error(), "checkpoint file") {
		t.Fatalf("checkpoint migration error = %v", err)
	}

	nonTransactional := checkedMigrationFS(t, map[string]string{
		"20260719000000_non_transactional.sql": "-- atlas:txmode none\n\nCREATE TABLE example (id bigint PRIMARY KEY);",
	})
	if _, err := loadDirectory(context.Background(), nonTransactional); err == nil ||
		!strings.Contains(err.Error(), "unsupported transaction mode") {
		t.Fatalf("non-transactional migration error = %v", err)
	}
}

func TestLoadDirectoryHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := loadDirectory(ctx, fstest.MapFS{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("load canceled directory error = %v, want context canceled", err)
	}
}

func checkedMigrationFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	directory := &atlasmigrate.MemDir{}
	source := make(fstest.MapFS, len(files)+1)
	for name, statement := range files {
		data := []byte(statement)
		if err := directory.WriteFile(name, data); err != nil {
			t.Fatalf("stage %s: %v", name, err)
		}
		source[name] = &fstest.MapFile{Data: data}
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum migration directory: %v", err)
	}
	data, err := checksum.MarshalText()
	if err != nil {
		t.Fatalf("marshal migration checksum: %v", err)
	}
	source[atlasmigrate.HashFileName] = &fstest.MapFile{Data: data}
	return source
}
