package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"

	"github.com/huynhanx03/judgify/internal/ent/desiredschema"
	entauthoring "github.com/huynhanx03/judgify/internal/ent/schema"
)

func TestSchemaGenerationIdentityIsUniqueAndExactlyOwned(t *testing.T) {
	ddl := submissionJudgeDDL(t)

	for _, fragment := range []string{
		`CREATE UNIQUE INDEX "judgespec_submission_id_generation" ON "judge_specs" ("submission_id", "generation")`,
		`CREATE UNIQUE INDEX "judgejob_submission_id_generation" ON "judge_jobs" ("submission_id", "generation")`,
		`CREATE UNIQUE INDEX "judgement_submission_id_generation" ON "submission_judgements" ("submission_id", "generation")`,
		`CONSTRAINT "judge_specs_submission_owner_fkey" FOREIGN KEY ("submission_id", "user_id", "problem_id") REFERENCES "submissions" ("id", "user_id", "problem_id")`,
		`CONSTRAINT "judge_jobs_spec_generation_fkey" FOREIGN KEY ("judge_spec_id", "submission_id", "generation") REFERENCES "judge_specs" ("id", "submission_id", "generation")`,
		`CONSTRAINT "judge_attempts_job_generation_spec_fkey" FOREIGN KEY ("job_id", "submission_id", "generation", "judge_spec_id") REFERENCES "judge_jobs" ("id", "submission_id", "generation", "judge_spec_id")`,
		`CONSTRAINT "submission_judgements_winning_attempt_fkey" FOREIGN KEY ("winning_attempt_id", "submission_id", "generation", "judge_spec_id") REFERENCES "judge_attempts" ("id", "submission_id", "generation", "judge_spec_id")`,
		`CONSTRAINT "submissions_active_judgement_fkey" FOREIGN KEY ("active_judgement_id", "id", "active_judgement_generation", "active_verdict") REFERENCES "submission_judgements" ("id", "submission_id", "generation", "verdict")`,
		`CONSTRAINT "submissions_contest_problem_snapshot_fkey" FOREIGN KEY ("contest_problem_id", "contest_id", "problem_id") REFERENCES "contest_problems" ("id", "contest_id", "problem_id")`,
	} {
		if !strings.Contains(ddl, fragment) {
			t.Errorf("desired schema is missing exact identity: %s", fragment)
		}
	}
}

func TestSchemaJudgementAndAttemptProvenanceIsSpecBound(t *testing.T) {
	ddl := submissionJudgeDDL(t)

	for _, fragment := range []string{
		`CONSTRAINT "judge_attempts_spec_provenance_fkey" FOREIGN KEY ("judge_spec_id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum") REFERENCES "judge_specs" ("id", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum")`,
		`CONSTRAINT "submission_judgements_spec_generation_fkey" FOREIGN KEY ("judge_spec_id", "submission_id", "generation", "testset_revision_id", "problem_revision_id", "judge_spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum", "release_id", "approved_release_manifest_checksum", "checker_implementation_checksum") REFERENCES "judge_specs" ("id", "submission_id", "generation", "testset_revision_id", "problem_revision_id", "spec_checksum", "reward_snapshot_checksum", "runtime_profile_version", "compiler_version", "runtime_bootstrap_version", "supervisor_protocol_version", "sandbox_image_digest", "artifact_manifest_checksum", "release_id", "approved_release_manifest_checksum", "checker_implementation_checksum")`,
	} {
		if !strings.Contains(ddl, fragment) {
			t.Errorf("desired schema is missing immutable provenance binding: %s", fragment)
		}
	}
}

func TestSchemaTestResultsBindAttemptSpecGroupAndExactTestKind(t *testing.T) {
	ddl := submissionJudgeDDL(t)

	for _, fragment := range []string{
		`CONSTRAINT "judge_test_results_attempt_spec_fkey" FOREIGN KEY ("attempt_id", "judge_spec_id") REFERENCES "judge_attempts" ("id", "judge_spec_id")`,
		`CONSTRAINT "judge_test_results_spec_testset_fkey" FOREIGN KEY ("judge_spec_id", "testset_revision_id") REFERENCES "judge_specs" ("id", "testset_revision_id")`,
		`CONSTRAINT "judge_test_results_group_identity_fkey" FOREIGN KEY ("group_id", "testset_revision_id", "group_index", "group_stop_on_failure") REFERENCES "testcase_groups" ("id", "testset_revision_id", "group_index", "stop_on_failure")`,
		`CONSTRAINT "judge_test_results_testcase_identity_fkey" FOREIGN KEY ("testcase_revision_id", "testset_revision_id", "group_id", "group_index", "case_order", "test_kind", "group_stop_on_failure") REFERENCES "testcase_revisions" ("id", "testset_revision_id", "group_id", "group_index", "order_index", "kind", "stop_on_failure")`,
	} {
		if !strings.Contains(ddl, fragment) {
			t.Errorf("desired schema is missing result ownership binding: %s", fragment)
		}
	}
	if !strings.Contains(
		schemaChecks(t, entauthoring.JudgeTestResult{}.Annotations())["judge_test_result_safe_diagnostic"],
		"test_kind = 'sample'",
	) {
		t.Fatal("safe diagnostics are not restricted to spec-bound sample tests")
	}
}

func TestSchemaLeaseSecretsAreUniqueHashesOnly(t *testing.T) {
	for name, fields := range map[string][]ent.Field{
		"job":     (entauthoring.JudgeJob{}).Fields(),
		"attempt": (entauthoring.JudgeAttempt{}).Fields(),
	} {
		hashFound := false
		for _, schemaField := range fields {
			descriptor := schemaField.Descriptor()
			if descriptor.Name == "lease_token" {
				t.Errorf("%s schema persists a raw lease token", name)
			}
			if descriptor.Name == "lease_token_hash" {
				hashFound = true
				if !descriptor.Sensitive {
					t.Errorf("%s lease token hash is not sensitive", name)
				}
			}
		}
		if !hashFound {
			t.Errorf("%s schema is missing lease_token_hash", name)
		}
	}
	ddl := submissionJudgeDDL(t)
	for _, fragment := range []string{
		`CREATE UNIQUE INDEX "judgejob_lease_token_hash" ON "judge_jobs" ("lease_token_hash")`,
		`CREATE UNIQUE INDEX "judgeattempt_lease_token_hash" ON "judge_attempts" ("lease_token_hash")`,
	} {
		if !strings.Contains(ddl, fragment) {
			t.Errorf("desired schema is missing unique lease hash index: %s", fragment)
		}
	}
	for _, check := range []string{
		schemaChecks(t, entauthoring.JudgeJob{}.Annotations())["judge_job_lease_hash_shape"],
		schemaChecks(t, entauthoring.JudgeAttempt{}.Annotations())["judge_attempt_lease_hash_shape"],
	} {
		if !strings.Contains(check, "octet_length(lease_token_hash) = 32") {
			t.Errorf("lease hash physical bound is missing from %q", check)
		}
	}
}

func TestSchemaSolveVersionsAndPointersAreDatabaseEnforced(t *testing.T) {
	transitionChecks := schemaChecks(t, entauthoring.UserProblemSolveTransition{}.Annotations())
	if got := transitionChecks["user_problem_solve_transition_contiguous"]; got != "to_version = from_version + 1" {
		t.Fatalf("contiguous version check = %q", got)
	}
	for name, value := range map[string]string{
		"projection": schemaChecks(t, entauthoring.UserProblemSolve{}.Annotations())["user_problem_solve_pointer_invariant"],
		"from":       transitionChecks["user_problem_solve_transition_from_side"],
		"to":         transitionChecks["user_problem_solve_transition_to_side"],
	} {
		for _, required := range []string{"status = 'unsolved'", "submission_id IS NULL", "status = 'solved'", "judgement_id IS NOT NULL"} {
			if !strings.Contains(value, required) {
				t.Errorf("%s pointer check %q is missing %q", name, value, required)
			}
		}
	}

	ddl := submissionJudgeDDL(t)
	for _, symbol := range []string{
		"user_problem_solves_active_submission_fkey",
		"user_problem_solves_active_judgement_fkey",
		"user_problem_solves_active_transition_fkey",
		"solve_transitions_solve_owner_fkey",
		"solve_transitions_from_judgement_fkey",
		"solve_transitions_to_judgement_fkey",
		"user_problem_solves_user_fkey",
		"user_problem_solves_problem_fkey",
	} {
		if !strings.Contains(ddl, `CONSTRAINT "`+symbol+`" FOREIGN KEY`) {
			t.Errorf("desired schema is missing solve ownership constraint %s", symbol)
		}
	}
	projectionChecks := schemaChecks(t, entauthoring.UserProblemSolve{}.Annotations())
	if !strings.Contains(projectionChecks["user_problem_solve_reward_checksum"], "octet_length(reward_facts_checksum) = 32") {
		t.Fatal("current solve reward checksum is not physically bounded")
	}
	if !strings.Contains(
		transitionChecks["user_problem_solve_transition_reward_checksums"],
		"octet_length(to_reward_facts_checksum) = 32",
	) {
		t.Fatal("transition solve reward checksums are not physically bounded")
	}
}

func TestSchemaSubmissionStatusKeepsProcessLimitVerdictExact(t *testing.T) {
	statusCheck := schemaChecks(
		t,
		entauthoring.Submission{}.Annotations(),
	)["submission_status_closed"]
	if !strings.Contains(statusCheck, "'process_limit_exceeded'") {
		t.Fatalf("submission status constraint collapses process limit: %q", statusCheck)
	}
}

func TestSchemaOwnerProblemHistoryHasSeekIndexes(t *testing.T) {
	indexes := (entauthoring.Submission{}).Indexes()
	for _, expected := range [][]string{
		{"user_id", "problem_id", "created_at", "id"},
		{"user_id", "problem_id", "contest_id", "created_at", "id"},
	} {
		found := false
		for _, schemaIndex := range indexes {
			descriptor := schemaIndex.Descriptor()
			if len(descriptor.Fields) != len(expected) {
				continue
			}
			matches := true
			for index, field := range expected {
				if descriptor.Fields[index] != field {
					matches = false
					break
				}
			}
			if matches {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("submission schema is missing owner history index %v", expected)
		}
	}
}

func submissionJudgeDDL(t *testing.T) string {
	t.Helper()
	t.Chdir("../../../../..")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	state, err := desiredschema.Load(ctx)
	if err != nil {
		t.Fatalf("load desired schema: %v", err)
	}
	ddl, err := state.RelationalDDL(ctx)
	if err != nil {
		t.Fatalf("emit desired schema: %v", err)
	}
	return ddl
}

func schemaChecks(t *testing.T, annotations []entschema.Annotation) map[string]string {
	t.Helper()
	for _, annotation := range annotations {
		if sqlAnnotation, ok := annotation.(*entsql.Annotation); ok {
			return sqlAnnotation.Checks
		}
	}
	t.Fatal("Ent SQL checks annotation is missing")
	return nil
}
