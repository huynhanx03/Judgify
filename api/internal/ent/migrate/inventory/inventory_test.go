package inventory_test

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"

	"github.com/huynhanx03/judgify/internal/ent/migrate/inventory"
)

type Widget struct {
	ent.Schema
}

func (Widget) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}),
	}
}

type MissingThing struct {
	ent.Schema
}

func (MissingThing) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}),
	}
}

func TestInspectReportsUUIDBigintMismatch(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_widgets.sql": {
				Data: []byte(`CREATE TABLE "public"."widgets" ("id" bigint PRIMARY KEY);`),
			},
		},
		[]ent.Interface{Widget{}},
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if len(report.IdentifierMismatches) != 1 {
		t.Fatalf("IdentifierMismatches = %#v, want one finding", report.IdentifierMismatches)
	}
	got := report.IdentifierMismatches[0]
	if got.Table != "widgets" || got.MigrationType != "bigint" || got.EntType != "uuid" {
		t.Fatalf("IdentifierMismatches[0] = %#v", got)
	}
}

func TestInspectReportsEntTableMissingFromMigrations(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_widgets.sql": {
				Data: []byte(`CREATE TABLE widgets (id uuid PRIMARY KEY);`),
			},
		},
		[]ent.Interface{Widget{}, MissingThing{}},
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if len(report.MissingMigrations) != 1 || report.MissingMigrations[0] != "missing_things" {
		t.Fatalf("MissingMigrations = %#v, want [missing_things]", report.MissingMigrations)
	}
}

func TestInspectReportsDroppedTableAndColumn(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_widgets.sql": {
				Data: []byte(`
CREATE TABLE widgets (id uuid PRIMARY KEY, legacy text);
ALTER TABLE widgets DROP COLUMN legacy;
DROP TABLE widgets;
`),
			},
		},
		[]ent.Interface{Widget{}},
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if len(report.DestructiveSQL) != 2 {
		t.Fatalf("DestructiveSQL = %#v, want two findings", report.DestructiveSQL)
	}
	if report.DestructiveSQL[0].Kind != inventory.FindingDropColumn {
		t.Fatalf("DestructiveSQL[0].Kind = %q", report.DestructiveSQL[0].Kind)
	}
	if report.DestructiveSQL[1].Kind != inventory.FindingDropTable {
		t.Fatalf("DestructiveSQL[1].Kind = %q", report.DestructiveSQL[1].Kind)
	}
}

func TestInspectReturnsStableOrderedOutput(t *testing.T) {
	migrations := fstest.MapFS{
		"020_second.sql": {
			Data: []byte(`CREATE TABLE zetas (id uuid PRIMARY KEY);`),
		},
		"010_first.sql": {
			Data: []byte(`CREATE TABLE alphas (id uuid PRIMARY KEY);`),
		},
	}

	first, err := inventory.Inspect(context.Background(), migrations, nil)
	if err != nil {
		t.Fatalf("first Inspect() error = %v", err)
	}
	second, err := inventory.Inspect(context.Background(), migrations, nil)
	if err != nil {
		t.Fatalf("second Inspect() error = %v", err)
	}

	if len(first.MigrationFiles) != 2 ||
		first.MigrationFiles[0].Name != "010_first.sql" ||
		first.MigrationFiles[1].Name != "020_second.sql" {
		t.Fatalf("MigrationFiles = %#v", first.MigrationFiles)
	}
	if len(first.DeclaredTables) != 2 ||
		first.DeclaredTables[0] != "alphas" ||
		first.DeclaredTables[1] != "zetas" {
		t.Fatalf("DeclaredTables = %#v", first.DeclaredTables)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprints differ: %q != %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestInspectReturnsTypedParseError(t *testing.T) {
	_, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_broken.sql": {
				Data: []byte(`CREATE TABLE widgets ("id" bigint;`),
			},
		},
		nil,
	)
	if err == nil {
		t.Fatal("Inspect() error = nil, want ParseError")
	}

	var parseErr *inventory.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("Inspect() error = %T %v, want *ParseError", err, err)
	}
	if parseErr.File != "001_broken.sql" {
		t.Fatalf("ParseError.File = %q", parseErr.File)
	}
}

func TestInspectHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := inventory.Inspect(ctx, fstest.MapFS{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Inspect() error = %v, want context.Canceled", err)
	}
}

func TestInspectRecordsUnknownSQLAsBoundedDiagnostic(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_unknown.sql": {
				Data: []byte(`VACUUM widgets;`),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %#v, want one finding", report.Diagnostics)
	}
	if report.Diagnostics[0].Kind != inventory.FindingUnknownSQL {
		t.Fatalf("Diagnostics[0].Kind = %q", report.Diagnostics[0].Kind)
	}
	err = report.Validate()
	var validationErr *inventory.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Diagnostics != 1 {
		t.Fatalf("Validate() error = %#v, want one diagnostic", err)
	}
}

func TestInspectRecognizesReviewedPostgreSQLFunctionAndTriggerDDL(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_append_only.sql": {
				Data: []byte(`
CREATE FUNCTION "public"."reject_mutation"()
RETURNS trigger
LANGUAGE plpgsql
AS $guard$
BEGIN
  RAISE EXCEPTION 'append-only';
END;
$guard$;

CREATE TRIGGER "events_append_only"
BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."events"
FOR EACH STATEMENT
EXECUTE FUNCTION "public"."reject_mutation"();
`),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want no unknown SQL", report.Diagnostics)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestInspectTreatsDroppedPostgreSQLFunctionsAndTriggersAsDestructive(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_drop_guards.sql": {
				Data: []byte(`
DROP TRIGGER events_append_only ON events;
DROP FUNCTION reject_mutation();
`),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.DestructiveSQL) != 2 {
		t.Fatalf("DestructiveSQL = %#v, want two findings", report.DestructiveSQL)
	}
	if report.DestructiveSQL[0].Kind != inventory.FindingDropTrigger ||
		report.DestructiveSQL[1].Kind != inventory.FindingDropFunction {
		t.Fatalf("DestructiveSQL = %#v", report.DestructiveSQL)
	}
}

func TestAnnotatedDestructiveSQLPassesValidation(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		fstest.MapFS{
			"001_cleanup.sql": {
				Data: []byte(`
CREATE TABLE widgets (id uuid PRIMARY KEY);
-- inventory:allow-destructive release-123 removes an unreleased table
DROP TABLE widgets;
`),
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(report.DestructiveSQL) != 1 {
		t.Fatalf("DestructiveSQL = %#v, want one finding", report.DestructiveSQL)
	}
	finding := report.DestructiveSQL[0]
	if !finding.Allowed || finding.Reason != "release-123 removes an unreleased table" {
		t.Fatalf("DestructiveSQL[0] = %#v", finding)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestReportValidateReturnsTypedError(t *testing.T) {
	report := inventory.Report{
		MissingMigrations: []string{"widgets"},
	}

	err := report.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}

	var validationErr *inventory.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error = %T %v, want *ValidationError", err, err)
	}
}

func TestCurrentMigrationBaselineMatchesEntSchemas(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		os.DirFS("../migrations"),
		inventory.CurrentSchemas(),
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if len(report.MigrationFiles) != 1 ||
		!strings.HasSuffix(report.MigrationFiles[0].Name, "_baseline.sql") {
		t.Fatalf("MigrationFiles = %#v, want one clean baseline", report.MigrationFiles)
	}
	if !slices.Contains(report.DeclaredTables, "casbin_rules") {
		t.Fatalf("DeclaredTables = %#v, want casbin_rules", report.DeclaredTables)
	}
	if !slices.Contains(report.EntTables, "casbin_rules") {
		t.Fatalf("EntTables = %#v, want Casbin adapter table owned by the schema", report.EntTables)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want no unknown SQL", report.Diagnostics)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v; report = %#v", err, report)
	}
}

func TestCheckedInMigrationInventoryReceiptIsCurrent(t *testing.T) {
	report, err := inventory.Inspect(
		context.Background(),
		os.DirFS("../migrations"),
		inventory.CurrentSchemas(),
	)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v; report = %#v", err, report)
	}

	var generated bytes.Buffer
	if err := inventory.WriteMarkdown(&generated, report); err != nil {
		t.Fatalf("WriteMarkdown() error = %v", err)
	}

	const receiptPath = "../../../../../docs/superpowers/baseline/2026-07-18-migration-inventory.md"
	checkedIn, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read migration inventory receipt %s: %v", receiptPath, err)
	}
	if !bytes.Equal(checkedIn, generated.Bytes()) {
		t.Fatalf(
			"migration inventory receipt is stale; regenerate it with:\n" +
				"  cd api && go run ./cmd/judgify inventory --format markdown > " +
				"../docs/superpowers/baseline/2026-07-18-migration-inventory.md",
		)
	}
}

func TestCurrentSchemasIncludesEveryEntSchemaDeclaration(t *testing.T) {
	want := declaredEntSchemaTypes(t, "../../schema")
	got := make([]string, 0, len(inventory.CurrentSchemas()))
	for _, schema := range inventory.CurrentSchemas() {
		typeOf := reflect.TypeOf(schema)
		if typeOf == nil {
			t.Fatal("CurrentSchemas() contains a nil schema")
		}
		if typeOf.Kind() == reflect.Pointer {
			typeOf = typeOf.Elem()
		}
		got = append(got, typeOf.Name())
	}
	sort.Strings(got)

	if !slices.Equal(got, want) {
		t.Fatalf("CurrentSchemas() types = %v, want every declared Ent schema %v", got, want)
	}
}

func declaredEntSchemaTypes(t *testing.T, schemaDir string) []string {
	t.Helper()

	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		t.Fatalf("read Ent schema directory: %v", err)
	}

	types := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filename := filepath.Join(schemaDir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatalf("parse Ent schema file %s: %v", filename, err)
		}

		entAliases := make(map[string]struct{})
		for _, importSpec := range file.Imports {
			importPath, err := strconv.Unquote(importSpec.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", filename, err)
			}
			if importPath != "entgo.io/ent" {
				continue
			}
			alias := "ent"
			if importSpec.Name != nil {
				alias = importSpec.Name.Name
			}
			entAliases[alias] = struct{}{}
		}

		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok || !embedsEntSchema(structure, entAliases) {
					continue
				}
				types = append(types, typeSpec.Name.Name)
			}
		}
	}

	sort.Strings(types)
	return types
}

func embedsEntSchema(structure *ast.StructType, entAliases map[string]struct{}) bool {
	for _, field := range structure.Fields.List {
		if len(field.Names) != 0 {
			continue
		}
		selector, ok := field.Type.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Schema" {
			continue
		}
		identifier, ok := selector.X.(*ast.Ident)
		if !ok {
			continue
		}
		if _, ok := entAliases[identifier.Name]; ok {
			return true
		}
	}
	return false
}
