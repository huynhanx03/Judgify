import assert from "node:assert/strict";
import { test } from "vitest";

import {
  progressionAdjustmentPreviewSchema,
  progressionLedgerPageSchema,
	  rankingReconciliationStartSchema,
	  rankingReconciliationPreviewSchema,
	  rankingReconciliationStatusSchema,
  rewardRuleSchema,
} from "@/lib/cultivation/progression-schema";

const ID = "019f6abb-8dd5-7581-8449-2b9ad77873e5";
const SOURCE_ID = "019f6abb-8dd5-7581-8449-2b9ad77873e6";
const CHECKSUM = "a".repeat(64);

test("progression ledger contracts keep immutable source and balance evidence", () => {
  const entry = {
    id: ID,
    kind: "solve_reward",
    source_type: "submission_verdict",
    source_aggregate_id: SOURCE_ID,
    source_version: 4,
    source_checksum: CHECKSUM,
    delta: 120,
    balance_before: 300,
    balance_after: 420,
    progression_version: 7,
    correlation_id: "corr-42",
    occurred_at: "2026-07-18T12:00:00.000Z",
  };
  assert.equal(
    progressionLedgerPageSchema.parse({ entries: [entry] }).entries[0]?.source_version,
    4,
  );
  assert.throws(
    () => progressionLedgerPageSchema.parse({ entries: [{ ...entry, source_version: 0 }] }),
    /invalid integer/,
  );
  assert.throws(
    () => progressionLedgerPageSchema.parse({ entries: [{ ...entry, mutable: true }] }),
    /unknown field/,
  );
  assert.throws(
    () => progressionLedgerPageSchema.parse({ entries: [{ ...entry, balance_after: 419 }] }),
    /balance evidence is inconsistent/,
  );
});

test("ranking reconciliation contracts expose bounded privacy-safe operation state", () => {
	const operation = {
		id: ID,
		kind: "cultivation.ranking.reconciliation.v1",
		status: "pending",
		total: 40,
		processed: 0,
		succeeded: 0,
		failed: 0,
		batch_size: 100,
		version: 1,
		requested_action: "none",
		attempt_count: 0,
		actor_user_id: SOURCE_ID,
		audit_id: "019f6abb-8dd5-7581-8449-2b9ad77873e7",
		correlation_id: "019f6abb-8dd5-7581-8449-2b9ad77873e8",
		available_at: "2026-07-18T12:00:00.000Z",
		created_at: "2026-07-18T12:00:00.000Z",
		updated_at: "2026-07-18T12:00:00.000Z",
	};
	const started = rankingReconciliationStartSchema.parse({
		operation,
		source_cutoff: "2026-07-18T12:00:00.000Z",
		source_count: 40,
		repair_count: 3,
		projection_revision: 8,
		idempotent_replay: false,
	});
	assert.equal(started.operation.total, 40);
	assert.equal(
		rankingReconciliationPreviewSchema.parse({
			source_cutoff: "2026-07-18T12:00:00.000Z",
			projection_revision: 8,
			source_count: 40,
			repair_count: 3,
			preview_checksum: CHECKSUM,
		}).repair_count,
		3,
	);
	assert.equal(
		rankingReconciliationStatusSchema.parse({
			projection_revision: 8,
			last_reconciled_at: "2026-07-18T12:00:00.000Z",
			latest_operation: operation,
		}).latest_operation?.correlation_id,
		"019f6abb-8dd5-7581-8449-2b9ad77873e8",
	);
	assert.throws(
		() => rankingReconciliationStatusSchema.parse({
			projection_revision: 8,
			latest_operation: { ...operation, request: { private: true } },
		}),
		/unknown field/,
	);
	assert.throws(
		() => rankingReconciliationStartSchema.parse({
			operation: { ...operation, kind: "other.operation" },
			source_cutoff: "2026-07-18T12:00:00.000Z",
			source_count: 40,
			repair_count: 3,
			projection_revision: 8,
			idempotent_replay: false,
		}),
		/ranking reconciliation operation/,
	);
});

test("adjustment and reward-rule contracts reject unbounded or unverifiable payloads", () => {
  assert.throws(
    () => progressionAdjustmentPreviewSchema.parse({
      user_id: ID,
      delta: 1,
      balance_before: 0,
      balance_after: 1,
      expected_version: 0,
      preview_checksum: "short",
    }),
    /SHA-256 checksum/,
  );
  assert.throws(
    () => rewardRuleSchema.parse({
      id: ID,
      rule_key: "solve_reward",
      revision: 1,
      algorithm_version: "v1",
      configuration: {
        multiplier_basis_points: 100_001,
        flat_bonus: 0,
      },
      checksum: CHECKSUM,
      effective_from: "2026-07-18T12:00:00.000Z",
    }),
    /invalid integer/,
  );
});
