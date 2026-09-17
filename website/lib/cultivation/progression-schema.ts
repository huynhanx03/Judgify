import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import {
	arraySchema,
	booleanSchema,
	integerSchema,
	optionalSchema,
	strictObjectSchema,
	stringSchema,
	type Schema,
} from "@/lib/api/schema";
import { operationSchema } from "@/lib/operations/operation-schema";
import { OPERATION_KIND } from "@/constants/operation";
import type {
	ProgressionAdjustmentPreview,
	ProgressionAdjustmentResult,
	ProgressionLedgerEntry,
	ProgressionLedgerPage,
	RewardRule,
	RewardRuleConfiguration,
	RewardRulePage,
	RewardRulePublicationPreview,
	RankingReconciliationStart,
	RankingReconciliationPreview,
	RankingReconciliationStatus,
} from "@/types/progression";

const SAFE_BALANCE = Number.MAX_SAFE_INTEGER;
const checksumSchema = stringSchema({
	minimumLength: 64,
	maximumLength: 64,
	pattern: /^[0-9a-f]{64}$/,
	label: "SHA-256 checksum",
});

const ledgerEntryDocumentSchema: Schema<ProgressionLedgerEntry> = strictObjectSchema({
	id: entityIDSchema,
	kind: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	source_type: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	source_aggregate_id: entityIDSchema,
	source_version: integerSchema({ minimum: 1 }),
	reversed_entry_id: optionalSchema(entityIDSchema),
	reward_rule_revision_id: optionalSchema(entityIDSchema),
	reward_profile_revision_id: optionalSchema(entityIDSchema),
	source_checksum: checksumSchema,
	delta: integerSchema({ minimum: -SAFE_BALANCE, maximum: SAFE_BALANCE }),
	balance_before: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	balance_after: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	progression_version: integerSchema({ minimum: 1 }),
	incident_id: optionalSchema(stringSchema({ maximumLength: 128 })),
	reason: optionalSchema(stringSchema({ maximumLength: 1024 })),
	correlation_id: stringSchema({ minimumLength: 1, maximumLength: 128 }),
	occurred_at: isoDateTimeSchema,
});

function requireExactBalance(
	value: { delta: number; balance_before: number; balance_after: number },
	path: string,
): void {
	if (value.balance_before + value.delta !== value.balance_after) {
		throw new TypeError(`${path}: balance evidence is inconsistent`);
	}
}

const ledgerEntrySchema: Schema<ProgressionLedgerEntry> = {
	parse(value: unknown, path = "$") {
	const entry = ledgerEntryDocumentSchema.parse(value, path);
	requireExactBalance(entry, path);
	if (
		(entry.reward_rule_revision_id === undefined) !==
		(entry.reward_profile_revision_id === undefined)
	) {
		throw new TypeError(`${path}: reward revision provenance is incomplete`);
	}
	return entry;
	},
};

export const progressionLedgerPageSchema: Schema<ProgressionLedgerPage> = strictObjectSchema({
	entries: arraySchema(ledgerEntrySchema, { maximumLength: 100 }),
	next_cursor: optionalSchema(entityIDSchema),
});

const progressionAdjustmentPreviewDocumentSchema: Schema<ProgressionAdjustmentPreview> = strictObjectSchema({
	user_id: entityIDSchema,
	delta: integerSchema({ minimum: -1_000_000, maximum: 1_000_000 }),
	balance_before: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	balance_after: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	expected_version: integerSchema({ minimum: 0 }),
	reversed_entry_id: optionalSchema(entityIDSchema),
	preview_checksum: checksumSchema,
});

export const progressionAdjustmentPreviewSchema: Schema<ProgressionAdjustmentPreview> = {
	parse(value: unknown, path = "$") {
		const preview = progressionAdjustmentPreviewDocumentSchema.parse(value, path);
		requireExactBalance(preview, path);
		return preview;
	},
};

const progressionAdjustmentResultDocumentSchema: Schema<ProgressionAdjustmentResult> = strictObjectSchema({
	ledger_entry_id: entityIDSchema,
	user_id: entityIDSchema,
	delta: integerSchema({ minimum: -1_000_000, maximum: 1_000_000 }),
	balance_before: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	balance_after: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	version: integerSchema({ minimum: 1 }),
	replayed: optionalSchema(booleanSchema),
	created_at: isoDateTimeSchema,
});

export const progressionAdjustmentResultSchema: Schema<ProgressionAdjustmentResult> = {
	parse(value: unknown, path = "$") {
		const result = progressionAdjustmentResultDocumentSchema.parse(value, path);
		requireExactBalance(result, path);
		return result;
	},
};

const rewardConfigurationSchema: Schema<RewardRuleConfiguration> = strictObjectSchema({
	multiplier_basis_points: integerSchema({ minimum: 0, maximum: 100_000 }),
	flat_bonus: integerSchema({ minimum: -1_000_000_000, maximum: 1_000_000_000 }),
});

export const rewardRuleSchema: Schema<RewardRule> = strictObjectSchema({
	id: entityIDSchema,
	rule_key: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	revision: integerSchema({ minimum: 1 }),
	algorithm_version: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	configuration: rewardConfigurationSchema,
	checksum: checksumSchema,
	effective_from: isoDateTimeSchema,
	replayed: optionalSchema(booleanSchema),
});

export const rewardRulePageSchema: Schema<RewardRulePage> = strictObjectSchema({
	rules: arraySchema(rewardRuleSchema, { maximumLength: 100 }),
	next_cursor: optionalSchema(integerSchema({ minimum: 1 })),
});

export const rewardRulePreviewSchema: Schema<RewardRulePublicationPreview> = strictObjectSchema({
	rule_key: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	current_revision: integerSchema({ minimum: 1 }),
	next_revision: integerSchema({ minimum: 2 }),
	algorithm_version: stringSchema({ minimumLength: 1, maximumLength: 64 }),
	configuration: rewardConfigurationSchema,
	preview_checksum: checksumSchema,
});

const rankingReconciliationStartDocumentSchema: Schema<RankingReconciliationStart> = strictObjectSchema({
	operation: operationSchema,
	source_cutoff: isoDateTimeSchema,
	source_count: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	repair_count: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	projection_revision: integerSchema({ minimum: 0 }),
	idempotent_replay: booleanSchema,
});

export const rankingReconciliationStartSchema: Schema<RankingReconciliationStart> = {
	parse(value: unknown, path = "$") {
		const result = rankingReconciliationStartDocumentSchema.parse(value, path);
		if (result.operation.kind !== OPERATION_KIND.CULTIVATION_RANKING_RECONCILIATION) {
			throw new TypeError(`${path}: expected ranking reconciliation operation`);
		}
		if (result.repair_count > result.source_count ||
			result.operation.total !== result.source_count) {
			throw new TypeError(`${path}: inconsistent ranking reconciliation counts`);
		}
		return result;
	},
};

const rankingReconciliationPreviewDocumentSchema: Schema<RankingReconciliationPreview> = strictObjectSchema({
	source_cutoff: isoDateTimeSchema,
	projection_revision: integerSchema({ minimum: 0 }),
	source_count: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	repair_count: integerSchema({ minimum: 0, maximum: SAFE_BALANCE }),
	preview_checksum: checksumSchema,
});

export const rankingReconciliationPreviewSchema: Schema<RankingReconciliationPreview> = {
	parse(value: unknown, path = "$") {
		const result = rankingReconciliationPreviewDocumentSchema.parse(value, path);
		if (result.repair_count > result.source_count) {
			throw new TypeError(`${path}: inconsistent ranking reconciliation counts`);
		}
		return result;
	},
};

const rankingReconciliationStatusDocumentSchema: Schema<RankingReconciliationStatus> = strictObjectSchema({
	projection_revision: integerSchema({ minimum: 0 }),
	last_reconciled_at: optionalSchema(isoDateTimeSchema),
	latest_operation: optionalSchema(operationSchema),
});

export const rankingReconciliationStatusSchema: Schema<RankingReconciliationStatus> = {
	parse(value: unknown, path = "$") {
		const result = rankingReconciliationStatusDocumentSchema.parse(value, path);
		if (result.latest_operation &&
			result.latest_operation.kind !== OPERATION_KIND.CULTIVATION_RANKING_RECONCILIATION) {
			throw new TypeError(`${path}: expected ranking reconciliation operation`);
		}
		return result;
	},
};
