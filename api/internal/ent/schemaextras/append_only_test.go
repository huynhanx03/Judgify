package schemaextras

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJudgifyAppendOnlyDDLIsDeterministicAndProtectsTheClosedCatalog(t *testing.T) {
	t.Parallel()

	first, err := JudgifyAppendOnlyDDL("public")
	require.NoError(t, err)
	second, err := JudgifyAppendOnlyDDL("public")
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, strings.Count(first, `CREATE FUNCTION "public"."judgify_reject_append_only_mutation"()`))
	require.Equal(t, 1, strings.Count(first, `USING ERRCODE = '55000'`))

	tables := AppendOnlyTables()
	require.Equal(t, []string{
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
	}, tables)
	require.NotContains(t, tables, "test_cases", "the legacy compatibility table remains mutable during cutover")
	for _, table := range tables {
		require.Equal(t, 1, strings.Count(first, `CREATE TRIGGER "`+table+`_append_only"`))
		require.Equal(t, 1, strings.Count(first,
			`BEFORE UPDATE OR DELETE OR TRUNCATE ON "public"."`+table+`"`,
		))
	}

	tables[0] = "mutated"
	require.Equal(t, "attribute_definition_revisions", AppendOnlyTables()[0], "catalog result must not alias package state")
}

func TestPostgreSQLAppendOnlyDDLRejectsUnsafeOrAmbiguousIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		function string
		tables   []string
	}{
		{name: "schema", schema: `public"; DROP TABLE users; --`, function: "guard", tables: []string{"events"}},
		{name: "function", schema: "public", function: "Guard", tables: []string{"events"}},
		{name: "empty tables", schema: "public", function: "guard"},
		{name: "table", schema: "public", function: "guard", tables: []string{"events;"}},
		{name: "duplicate table", schema: "public", function: "guard", tables: []string{"events", "events"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := PostgreSQLAppendOnlyDDL(test.schema, test.function, test.tables...)
			require.Error(t, err)
		})
	}
}
