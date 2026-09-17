package migration

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/huynhanx03/judgify/internal/ent/schemaextras"
	problemcontract "github.com/huynhanx03/judgify/internal/problem/contract"
)

const problemRevisionBaselinePath = "migrations/20260722073038_baseline.sql"

var problemRevisionImmutableTables = []string{
	"difficulty_revisions",
	"element_revisions",
	"problem_revisions",
	"tag_revision_elements",
	"tag_revisions",
	"test_assets",
	"testcase_groups",
	"testcase_revisions",
	"testset_revisions",
}

func TestProblemRevisionBaselineContainsImmutableTablesAndBounds(t *testing.T) {
	t.Parallel()

	sql := readProblemRevisionBaseline(t)
	requiredChecks := map[string][]string{
		"difficulty_revisions": {
			"difficulty_revision_checksum_shape",
			"difficulty_revision_nonzero_identity",
			"difficulty_revision_number_bound",
			"difficulty_revision_exp_reward_bound",
		},
		"element_revisions": {
			"element_revision_checksum_shape",
			"element_revision_nonzero_identity",
			"element_revision_number_bound",
		},
		"problem_revisions": {
			"problem_revision_checker_config_bound",
			"problem_revision_checker_identity_bound",
			"problem_revision_checksum_shape",
			"problem_revision_element_snapshot_complete",
			"problem_revision_limit_bounds",
			"problem_revision_nonzero_checksums",
			"problem_revision_nonzero_identity",
			"problem_revision_number_bound",
			"problem_revision_reward_mapping_bound",
			"problem_revision_reward_snapshot_bound",
			"problem_revision_runtime_keys_bound",
			"problem_revision_source_kind",
			"problem_revision_statement_bound",
			"problem_revision_title_bound",
			"problem_revision_wall_covers_cpu",
		},
		"tag_revisions": {
			"tag_revision_checksum_shape",
			"tag_revision_nonzero_identity",
			"tag_revision_number_bound",
		},
		"tag_revision_elements": {
			"tag_revision_element_nonzero_identity",
			"tag_revision_element_revision_bound",
		},
		"test_assets": {
			"test_asset_checksum_shape",
			"test_asset_content_bound",
			"test_asset_encoding_closed",
			"test_asset_encoding_size_consistent",
			"test_asset_nonzero_identity",
		},
		"testcase_groups": {
			"testcase_group_index_bound",
			"testcase_group_nonzero_identity",
		},
		"testcase_revisions": {
			"testcase_revision_group_index_bound",
			"testcase_revision_kind_closed",
			"testcase_revision_launch_weight",
			"testcase_revision_nonzero_identity",
			"testcase_revision_order_index_bound",
		},
		"testset_revisions": {
			"testset_revision_checksum_shape",
			"testset_revision_count_bound",
			"testset_revision_nonzero_identity",
			"testset_revision_number_bound",
			"testset_revision_total_size_bound",
		},
	}

	for _, table := range problemRevisionImmutableTables {
		table := table
		t.Run(table, func(t *testing.T) {
			t.Parallel()

			body := migrationTableBody(t, sql, table)
			for _, constraint := range requiredChecks[table] {
				needle := `CONSTRAINT "` + constraint + `" CHECK`
				if count := strings.Count(body, needle); count != 1 {
					t.Errorf("%s contains %q %d times, want exactly once", table, constraint, count)
				}
			}
			for _, mutableColumn := range []string{
				`"updated_at"`,
				`"deleted_at"`,
				`"deleted_by"`,
			} {
				if strings.Contains(body, mutableColumn) {
					t.Errorf("%s exposes mutable lifecycle column %s", table, mutableColumn)
				}
			}
		})
	}
}

func TestProblemBaselineKeepsLegacyVisibilityDerivedFromLifecycle(t *testing.T) {
	t.Parallel()

	body := migrationTableBody(t, readProblemRevisionBaseline(t), "problems")
	const constraint = `CONSTRAINT "problem_public_visibility_matches_lifecycle" CHECK (is_published = ((lifecycle)::text = 'published'::text))`
	if count := strings.Count(body, constraint); count != 1 {
		t.Fatalf("problems contains lifecycle visibility constraint %d times, want exactly once", count)
	}
}

func TestProblemBaselineRequiresStableSlug(t *testing.T) {
	t.Parallel()

	sql := readProblemRevisionBaseline(t)
	body := migrationTableBody(t, sql, "problems")
	if count := strings.Count(body, `"slug" character varying NOT NULL`); count != 1 {
		t.Fatalf("problems required slug column count = %d, want exactly once", count)
	}
	if strings.Contains(body, "slug IS NULL") {
		t.Fatal("problem slug constraint still permits NULL")
	}
	index := `CREATE UNIQUE INDEX "` +
		problemcontract.ProblemSlugUniqueConstraintName +
		`" ON "public"."problems" ("slug");`
	if count := strings.Count(sql, index); count != 1 {
		t.Fatalf(
			"stable slug unique constraint %q count = %d, want exactly once",
			problemcontract.ProblemSlugUniqueConstraintName,
			count,
		)
	}
}

func TestProblemBaselineContainsStableTaxonomyMappings(t *testing.T) {
	t.Parallel()

	raw := readProblemRevisionBaseline(t)
	sql := compactMigrationSQL(raw)
	problems := migrationTableBody(t, raw, "problems")
	for _, required := range []string{
		`"tag_mapping_id" uuid NOT NULL`,
		`"element_mapping_id" uuid NOT NULL`,
		`CONSTRAINT "problem_nonzero_mapping_identities" CHECK`,
		`tag_mapping_id <> element_mapping_id`,
	} {
		if !strings.Contains(problems, required) {
			t.Errorf("problems is missing stable taxonomy mapping contract %q", required)
		}
	}
	for table, constraint := range map[string]string{
		"difficulties": "difficulty_positive_version",
		"elements":     "element_positive_version",
		"tags":         "tag_positive_version",
	} {
		body := migrationTableBody(t, raw, table)
		if !strings.Contains(body, `"version" bigint NOT NULL DEFAULT 1`) {
			t.Errorf("%s is missing stable catalog version", table)
		}
		if !strings.Contains(body, `CONSTRAINT "`+constraint+`" CHECK`) {
			t.Errorf("%s is missing positive version constraint %q", table, constraint)
		}
	}

	indexes := []struct {
		table string
		cols  []string
	}{
		{"problems", []string{"id", "tag_mapping_id"}},
		{"problems", []string{"id", "element_mapping_id"}},
		{"difficulty_revisions", []string{"difficulty_id", "revision"}},
		{"element_revisions", []string{"element_id", "revision"}},
		{"tag_revisions", []string{"tag_id", "revision"}},
		{"tag_revision_elements", []string{"tag_revision_id", "element_id"}},
	}
	for _, index := range indexes {
		pattern := `CREATE UNIQUE INDEX "[^"]+" ON "public"\."` + regexp.QuoteMeta(index.table) +
			`" \(` + quotedColumnPattern(index.cols) + `\);`
		if count := len(regexp.MustCompile(pattern).FindAllString(sql, -1)); count != 1 {
			t.Errorf("%s unique index (%s) count = %d, want one", index.table, strings.Join(index.cols, ", "), count)
		}
	}
}

func TestProblemRevisionBaselineContainsOwnershipAndOrderingIndexes(t *testing.T) {
	t.Parallel()

	sql := compactMigrationSQL(readProblemRevisionBaseline(t))
	indexes := []struct {
		table   string
		columns []string
		unique  bool
	}{
		{table: "problem_revisions", columns: []string{"id", "problem_id"}, unique: true},
		{table: "problem_revisions", columns: []string{"id", "render_source_id", "render_source_kind", "render_source_checksum"}, unique: true},
		{table: "problem_revisions", columns: []string{"problem_id", "revision_number"}, unique: true},
		{table: "problem_revisions", columns: []string{"render_source_id", "render_source_kind"}, unique: true},
		{table: "test_assets", columns: []string{"sha256"}, unique: true},
		{table: "testcase_groups", columns: []string{"id", "testset_revision_id", "group_index", "stop_on_failure"}, unique: true},
		{table: "testcase_groups", columns: []string{"testset_revision_id", "group_index"}, unique: true},
		{table: "testcase_groups", columns: []string{"testset_revision_id", "group_index", "stop_on_failure"}, unique: true},
		{table: "testcase_revisions", columns: []string{"group_id", "order_index"}, unique: true},
		{table: "testcase_revisions", columns: []string{"testset_revision_id", "group_index", "order_index"}, unique: true},
		{table: "testcase_revisions", columns: []string{"testset_revision_id", "test_case_id"}, unique: true},
		{table: "testcase_revisions", columns: []string{"test_case_id", "problem_id"}},
		{table: "testcase_revisions", columns: []string{"testset_revision_id", "problem_id"}},
		{table: "testcase_revisions", columns: []string{"group_id", "testset_revision_id", "group_index", "stop_on_failure"}},
		{table: "testset_revisions", columns: []string{"id", "problem_id"}, unique: true},
		{table: "testset_revisions", columns: []string{"problem_id", "revision_number"}, unique: true},
	}

	for _, index := range indexes {
		prefix := `CREATE INDEX "[^"]+"`
		if index.unique {
			prefix = `CREATE UNIQUE INDEX "[^"]+"`
		}
		pattern := prefix + ` ON "public"\."` + regexp.QuoteMeta(index.table) +
			`" \(` + quotedColumnPattern(index.columns) + `\);`
		matches := regexp.MustCompile(pattern).FindAllString(sql, -1)
		if len(matches) != 1 {
			t.Errorf(
				"%s index on (%s), unique=%t: found %d, want exactly one",
				index.table,
				strings.Join(index.columns, ", "),
				index.unique,
				len(matches),
			)
		}
	}
}

func TestProblemRevisionBaselineContainsAllCompositeIdentityForeignKeys(t *testing.T) {
	t.Parallel()

	sql := compactMigrationSQL(readProblemRevisionBaseline(t))
	foreignKeys := []struct {
		name       string
		source     string
		columns    []string
		target     string
		refColumns []string
	}{
		{
			name:       "problem_revisions_render_source_identity_fkey",
			source:     "problem_revisions",
			columns:    []string{"render_source_id", "id", "render_source_kind", "render_source_checksum"},
			target:     "content_render_sources",
			refColumns: []string{"id", "owner_revision_id", "source_kind", "source_checksum"},
		},
		{
			name:       "problem_revisions_testset_owner_fkey",
			source:     "problem_revisions",
			columns:    []string{"testset_revision_id", "problem_id"},
			target:     "testset_revisions",
			refColumns: []string{"id", "problem_id"},
		},
		{
			name:       "tag_revision_elements_element_revision_fkey",
			source:     "tag_revision_elements",
			columns:    []string{"element_id", "element_revision"},
			target:     "element_revisions",
			refColumns: []string{"element_id", "revision"},
		},
		{
			name:       "problem_revisions_difficulty_revision_fkey",
			source:     "problem_revisions",
			columns:    []string{"difficulty_id", "difficulty_revision"},
			target:     "difficulty_revisions",
			refColumns: []string{"difficulty_id", "revision"},
		},
		{
			name:       "problem_revisions_tag_mapping_owner_fkey",
			source:     "problem_revisions",
			columns:    []string{"problem_id", "tag_mapping_id"},
			target:     "problems",
			refColumns: []string{"id", "tag_mapping_id"},
		},
		{
			name:       "problem_revisions_element_mapping_owner_fkey",
			source:     "problem_revisions",
			columns:    []string{"problem_id", "element_mapping_id"},
			target:     "problems",
			refColumns: []string{"id", "element_mapping_id"},
		},
		{
			name:       "problems_draft_revision_source_fkey",
			source:     "problems",
			columns:    []string{"draft_revision_id", "draft_render_source_id", "draft_render_source_kind", "draft_render_source_checksum"},
			target:     "problem_revisions",
			refColumns: []string{"id", "render_source_id", "render_source_kind", "render_source_checksum"},
		},
		{
			name:       "problems_draft_artifact_source_fkey",
			source:     "problems",
			columns:    []string{"draft_render_artifact_id", "draft_render_source_id", "draft_render_source_kind", "draft_render_source_checksum"},
			target:     "content_render_artifacts",
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			name:       "problems_draft_revision_owner_fkey",
			source:     "problems",
			columns:    []string{"draft_revision_id", "id"},
			target:     "problem_revisions",
			refColumns: []string{"id", "problem_id"},
		},
		{
			name:       "problems_published_revision_source_fkey",
			source:     "problems",
			columns:    []string{"published_revision_id", "published_render_source_id", "published_render_source_kind", "published_render_source_checksum"},
			target:     "problem_revisions",
			refColumns: []string{"id", "render_source_id", "render_source_kind", "render_source_checksum"},
		},
		{
			name:       "problems_published_artifact_source_fkey",
			source:     "problems",
			columns:    []string{"published_render_artifact_id", "published_render_source_id", "published_render_source_kind", "published_render_source_checksum"},
			target:     "content_render_artifacts",
			refColumns: []string{"id", "render_source_id", "render_source_kind", "source_checksum"},
		},
		{
			name:       "problems_published_revision_owner_fkey",
			source:     "problems",
			columns:    []string{"published_revision_id", "id"},
			target:     "problem_revisions",
			refColumns: []string{"id", "problem_id"},
		},
		{
			name:       "testcase_revisions_test_case_owner_fkey",
			source:     "testcase_revisions",
			columns:    []string{"test_case_id", "problem_id"},
			target:     "test_cases",
			refColumns: []string{"id", "problem_id"},
		},
		{
			name:       "testcase_revisions_testset_owner_fkey",
			source:     "testcase_revisions",
			columns:    []string{"testset_revision_id", "problem_id"},
			target:     "testset_revisions",
			refColumns: []string{"id", "problem_id"},
		},
		{
			name:       "testcase_revisions_group_identity_fkey",
			source:     "testcase_revisions",
			columns:    []string{"group_id", "testset_revision_id", "group_index", "stop_on_failure"},
			target:     "testcase_groups",
			refColumns: []string{"id", "testset_revision_id", "group_index", "stop_on_failure"},
		},
	}

	for _, foreignKey := range foreignKeys {
		clause := `CONSTRAINT "` + foreignKey.name + `" FOREIGN KEY (` +
			quotedColumns(foreignKey.columns) + `) REFERENCES "public"."` +
			foreignKey.target + `" (` + quotedColumns(foreignKey.refColumns) +
			`) ON UPDATE NO ACTION ON DELETE NO ACTION`
		if count := strings.Count(sql, clause); count != 1 {
			t.Errorf("composite foreign key %s found %d times, want exactly once", foreignKey.name, count)
		}
		statement := migrationAlterTableStatement(t, sql, foreignKey.source)
		if count := strings.Count(statement, clause); count != 1 {
			t.Errorf(
				"composite foreign key %s is attached to %s %d times, want exactly once",
				foreignKey.name,
				foreignKey.source,
				count,
			)
		}
	}
}

func TestProblemRevisionBaselineHasExactAppendOnlyGuardCatalog(t *testing.T) {
	t.Parallel()

	sql := compactMigrationSQL(readProblemRevisionBaseline(t))
	const function = `"public"."judgify_reject_append_only_mutation"`
	if count := strings.Count(sql, `CREATE FUNCTION `+function+`()`); count != 1 {
		t.Fatalf("append-only function definition count = %d, want exactly one", count)
	}
	for _, required := range []string{
		`RETURNS trigger LANGUAGE plpgsql`,
		`RAISE EXCEPTION 'table % is append-only', TG_TABLE_NAME USING ERRCODE = '55000'`,
		`RETURN NULL`,
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("append-only function is missing %q", required)
		}
	}

	// Keep this list independent of the schema-extra implementation: the test
	// must fail if either the reviewed catalog or generated baseline changes
	// without an explicit migration-contract review.
	expected := []string{
		"attribute_definition_revisions",
		"audit_logs",
		"content_render_artifacts",
		"content_render_sources",
		"contest_content_revisions",
		"contest_problems",
		"contest_registration_transitions",
		"contest_transitions",
		"cultivation_catalog_revisions",
		"cultivation_ranking_privacy_changes",
		"difficulty_revisions",
		"element_revisions",
		"judge_specs",
		"judge_test_results",
		"judge_worker_capabilities",
		"material_revision_tags",
		"material_revisions",
		"notification_campaign_explicit_users",
		"notification_campaign_revisions",
		"onboarding_trait_offer_talents",
		"operation_batches",
		"operation_control_receipts",
		"problem_revisions",
		"progression_ledger_entries",
		"reward_rule_revisions",
		"solve_reward_processed_transitions",
		"submission_judgements",
		"tag_revision_elements",
		"tag_revisions",
		"test_assets",
		"testcase_groups",
		"testcase_revisions",
		"testset_revisions",
		"trait_effect_revisions",
		"user_problem_solve_transitions",
		"user_reward_profile_revisions",
		"user_reward_profile_traits",
	}
	pattern := regexp.MustCompile(
		`CREATE TRIGGER "([^"]+)_append_only" ` +
			`BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"\."([^"]+)" ` +
			`FOR EACH STATEMENT EXECUTE FUNCTION ` + regexp.QuoteMeta(function) + `\(\);`,
	)
	matches := pattern.FindAllStringSubmatch(sql, -1)
	if len(matches) != len(expected) {
		t.Fatalf("well-formed append-only trigger count = %d, want %d", len(matches), len(expected))
	}

	actual := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		triggerStem, table := match[1], match[2]
		if triggerStem != table {
			t.Errorf("append-only trigger %s_append_only protects table %s", triggerStem, table)
		}
		if _, duplicate := seen[table]; duplicate {
			t.Errorf("append-only table %s is guarded more than once", table)
		}
		seen[table] = struct{}{}
		actual = append(actual, table)
	}
	sort.Strings(actual)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		t.Errorf("append-only tables = %v, want %v", actual, expected)
	}

	appendOnlyNames := regexp.MustCompile(`CREATE TRIGGER "[^"]+_append_only"`).FindAllString(sql, -1)
	if len(appendOnlyNames) != len(expected) {
		t.Errorf("append-only trigger declarations = %d, want exactly %d", len(appendOnlyNames), len(expected))
	}
}

func TestProblemRevisionBaselineHasConditionalAppendOnlyRoleACL(t *testing.T) {
	t.Parallel()

	sql := compactMigrationSQL(readProblemRevisionBaseline(t))
	canonical, err := schemaextras.JudgifyAppendOnlyACLDDL("public")
	if err != nil {
		t.Fatalf("build canonical append-only ACL: %v", err)
	}
	if count := strings.Count(readProblemRevisionBaseline(t), canonical); count != 1 {
		t.Errorf("baseline canonical append-only ACL count = %d, want one", count)
	}
	for _, required := range []string{
		`DO $$`,
		`FROM pg_roles WHERE rolname = 'judgify_api'`,
		`FROM pg_roles WHERE rolname = 'judgify_worker'`,
		`REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."audit_logs" FROM judgify_api`,
		`REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."audit_logs" FROM judgify_worker`,
		`REVOKE INSERT ON TABLE "public"."difficulty_revisions" FROM judgify_worker`,
		`REVOKE INSERT ON TABLE "public"."element_revisions" FROM judgify_worker`,
		`REVOKE INSERT ON TABLE "public"."tag_revisions" FROM judgify_worker`,
		`REVOKE INSERT ON TABLE "public"."tag_revision_elements" FROM judgify_worker`,
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("baseline is missing conditional append-only ACL contract %q", required)
		}
	}
	for _, table := range problemRevisionImmutableTables {
		for _, role := range []string{"judgify_api", "judgify_worker"} {
			statement := `REVOKE UPDATE, DELETE, TRUNCATE ON TABLE "public"."` + table + `" FROM ` + role
			if count := strings.Count(sql, statement); count != 1 {
				t.Errorf("ACL statement %q count = %d, want one", statement, count)
			}
		}
	}
}

func TestProblemRevisionImmutableTablesHaveNoCascadingOrMutablePaths(t *testing.T) {
	t.Parallel()

	raw := readProblemRevisionBaseline(t)
	immutable := make(map[string]struct{}, len(problemRevisionImmutableTables))
	for _, table := range problemRevisionImmutableTables {
		immutable[table] = struct{}{}
		body := migrationTableBody(t, raw, table)
		upperBody := strings.ToUpper(body)
		for _, forbidden := range []string{
			"ON UPDATE CASCADE",
			"ON UPDATE SET NULL",
			"ON UPDATE SET DEFAULT",
			"ON DELETE CASCADE",
			"ON DELETE SET NULL",
			"ON DELETE SET DEFAULT",
		} {
			if strings.Contains(upperBody, forbidden) {
				t.Errorf("%s contains mutable foreign-key action %q", table, forbidden)
			}
		}
	}

	sql := compactMigrationSQL(raw)
	alterPattern := regexp.MustCompile(`ALTER TABLE "public"\."([^"]+)" (.*?);`)
	foreignKeyPattern := regexp.MustCompile(
		`CONSTRAINT "([^"]+)" FOREIGN KEY \([^)]*\) REFERENCES "public"\."([^"]+)" \([^)]*\) ` +
			`ON UPDATE (NO ACTION|RESTRICT|CASCADE|SET NULL|SET DEFAULT) ` +
			`ON DELETE (NO ACTION|RESTRICT|CASCADE|SET NULL|SET DEFAULT)`,
	)
	touched := make(map[string]bool, len(immutable))
	for _, alter := range alterPattern.FindAllStringSubmatch(sql, -1) {
		source, clauses := alter[1], alter[2]
		for _, foreignKey := range foreignKeyPattern.FindAllStringSubmatch(clauses, -1) {
			name, target := foreignKey[1], foreignKey[2]
			_, sourceImmutable := immutable[source]
			_, targetImmutable := immutable[target]
			if !sourceImmutable && !targetImmutable {
				continue
			}
			touched[source] = touched[source] || sourceImmutable
			touched[target] = touched[target] || targetImmutable
			if foreignKey[3] != "NO ACTION" || foreignKey[4] != "NO ACTION" {
				t.Errorf(
					"foreign key %s (%s -> %s) has ON UPDATE %s ON DELETE %s; immutable relations require NO ACTION",
					name,
					source,
					target,
					foreignKey[3],
					foreignKey[4],
				)
			}
		}
	}
	for _, table := range problemRevisionImmutableTables {
		if !touched[table] {
			t.Errorf("did not inspect any foreign-key path touching immutable table %s", table)
		}
	}
}

func readProblemRevisionBaseline(t *testing.T) string {
	t.Helper()

	data, err := embeddedMigrations.ReadFile(problemRevisionBaselinePath)
	if err != nil {
		t.Fatalf("read embedded Task2 baseline: %v", err)
	}
	return canonicalPublicSchemaSQL(string(data))
}

// canonicalPublicSchemaSQL normalizes Atlas' two equivalent PostgreSQL
// renderings for the configured public search_path. Recent Atlas versions
// omit the explicit schema on relational objects while the reviewed schema
// extras remain explicitly qualified. Keeping assertions in one canonical
// representation lets the tests continue to verify exact tables, indexes and
// foreign keys instead of coupling those invariants to printer formatting.
func canonicalPublicSchemaSQL(sql string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`CREATE TABLE "([^"]+)" \(`),
		regexp.MustCompile(`ALTER TABLE "([^"]+)" `),
		regexp.MustCompile(`REFERENCES "([^"]+)" \(`),
		regexp.MustCompile(`(CREATE(?: UNIQUE)? INDEX "[^"]+" ON) "([^"]+)" \(`),
	}
	sql = patterns[0].ReplaceAllString(sql, `CREATE TABLE "public"."$1" (`)
	sql = patterns[1].ReplaceAllString(sql, `ALTER TABLE "public"."$1" `)
	sql = patterns[2].ReplaceAllString(sql, `REFERENCES "public"."$1" (`)
	return patterns[3].ReplaceAllString(sql, `$1 "public"."$2" (`)
}

func migrationTableBody(t *testing.T, sql, table string) string {
	t.Helper()

	startToken := `CREATE TABLE "public"."` + table + `" (`
	if count := strings.Count(sql, startToken); count != 1 {
		t.Fatalf("%s create-table count = %d, want exactly one", table, count)
	}
	start := strings.Index(sql, startToken)
	end := strings.Index(sql[start:], "\n);")
	if end == -1 {
		t.Fatalf("%s create-table statement is not terminated", table)
	}
	return sql[start : start+end+3]
}

func compactMigrationSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

func migrationAlterTableStatement(t *testing.T, sql, table string) string {
	t.Helper()

	startToken := `ALTER TABLE "public"."` + table + `" `
	if count := strings.Count(sql, startToken); count != 1 {
		t.Fatalf("%s alter-table count = %d, want exactly one", table, count)
	}
	start := strings.Index(sql, startToken)
	end := strings.Index(sql[start:], ";")
	if end == -1 {
		t.Fatalf("%s alter-table statement is not terminated", table)
	}
	return sql[start : start+end+1]
}

func quotedColumns(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, fmt.Sprintf(`"%s"`, column))
	}
	return strings.Join(quoted, ", ")
}

func quotedColumnPattern(columns []string) string {
	return regexp.QuoteMeta(quotedColumns(columns))
}
