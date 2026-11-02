// Package schemaextras owns reviewed PostgreSQL objects that Ent and the
// open-source Atlas schema inspector cannot model.
package schemaextras

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const appendOnlyFunctionName = "judgify_reject_append_only_mutation"

var (
	postgresIdentifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
	judgifyAppendOnlyTables   = []string{
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
)

// JudgifyAppendOnlyDDL returns the canonical trigger function and trigger set
// for every durable append-only table. schema is empty for Atlas desired-state
// SQL and "public" for the checked migration baseline.
func JudgifyAppendOnlyDDL(schema string) (string, error) {
	return PostgreSQLAppendOnlyDDL(schema, appendOnlyFunctionName, judgifyAppendOnlyTables...)
}

// AppendOnlyTables returns an owned copy of the protected table catalog.
func AppendOnlyTables() []string {
	return append([]string(nil), judgifyAppendOnlyTables...)
}

// JudgifyMigrationFooterDDL returns every reviewed PostgreSQL object that the
// Atlas OSS relational diff cannot model. Baseline generation must append this
// value as one unit so trigger coverage and least-privilege grants cannot
// drift into independently maintained SQL fragments.
func JudgifyMigrationFooterDDL(schema string) (string, error) {
	appendOnlyDDL, err := JudgifyAppendOnlyDDL(schema)
	if err != nil {
		return "", err
	}
	aclDDL, err := JudgifyAppendOnlyACLDDL(schema)
	if err != nil {
		return "", err
	}
	return appendOnlyDDL + "\n\n" + aclDDL, nil
}

// PostgreSQLAppendOnlyDDL builds deterministic statement-level guards.
// Statement-level triggers reject UPDATE/DELETE/TRUNCATE even when no row
// matches, and PostgreSQL also invokes UPDATE triggers for an
// INSERT ... ON CONFLICT DO UPDATE path.
func PostgreSQLAppendOnlyDDL(schema, function string, tables ...string) (string, error) {
	if schema != "" && !postgresIdentifierPattern.MatchString(schema) {
		return "", errors.New("schema extras: invalid PostgreSQL schema identifier")
	}
	if !postgresIdentifierPattern.MatchString(function) {
		return "", errors.New("schema extras: invalid PostgreSQL function identifier")
	}
	if len(tables) == 0 {
		return "", errors.New("schema extras: append-only table catalog is empty")
	}

	ordered := append([]string(nil), tables...)
	sort.Strings(ordered)
	for index, table := range ordered {
		if !postgresIdentifierPattern.MatchString(table) || len(table+"_append_only") > 63 {
			return "", fmt.Errorf("schema extras: invalid append-only table identifier %q", table)
		}
		if index > 0 && table == ordered[index-1] {
			return "", fmt.Errorf("schema extras: duplicate append-only table %q", table)
		}
	}

	qualifiedFunction := qualify(schema, function)
	var ddl strings.Builder
	fmt.Fprintf(&ddl, `-- Enforce append-only storage outside Ent.
CREATE FUNCTION %s()
RETURNS trigger
LANGUAGE plpgsql
AS $judgify_append_only$
BEGIN
  RAISE EXCEPTION 'table %% is append-only', TG_TABLE_NAME
    USING ERRCODE = '55000';
  RETURN NULL;
END;
$judgify_append_only$;
`, qualifiedFunction)
	for _, table := range ordered {
		fmt.Fprintf(&ddl, `
CREATE TRIGGER %s
BEFORE UPDATE OR DELETE OR TRUNCATE ON %s
FOR EACH STATEMENT
EXECUTE FUNCTION %s();
`, quoteIdentifier(table+"_append_only"), qualify(schema, table), qualifiedFunction)
	}
	return strings.TrimSpace(ddl.String()), nil
}

func qualify(schema, name string) string {
	if schema == "" {
		return quoteIdentifier(name)
	}
	return quoteIdentifier(schema) + "." + quoteIdentifier(name)
}

func quoteIdentifier(value string) string {
	return `"` + value + `"`
}
