import { ADMIN_CULTIVATION_API } from "@/constants/api/cultivation";
import { api } from "@/lib/api/client";
import { commandPurpose } from "@/lib/api/command-purpose";
import { entityIDSchema } from "@/lib/api/contracts";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import {
	progressionAdjustmentPreviewSchema,
	progressionAdjustmentResultSchema,
	progressionLedgerPageSchema,
	rankingReconciliationPreviewSchema,
	rankingReconciliationStartSchema,
	rankingReconciliationStatusSchema,
	rewardRulePageSchema,
	rewardRulePreviewSchema,
	rewardRuleSchema,
} from "@/lib/cultivation/progression-schema";
import type {
	ApplyProgressionAdjustmentInput,
	ProgressionAdjustmentPreview,
	ProgressionAdjustmentResult,
	ProgressionLedgerPage,
	PublishRewardRuleInput,
	RewardRule,
	RewardRuleConfiguration,
	RewardRulePage,
	RewardRulePublicationPreview,
	RankingReconciliationStart,
	RankingReconciliationPreview,
	RankingReconciliationStatus,
} from "@/types/progression";

function ledgerQuery(userID: string, cursor?: string, limit = 25): string {
	const parameters = new URLSearchParams();
	parameters.set("user_id", entityIDSchema.parse(userID));
	if (cursor) parameters.set("cursor", entityIDSchema.parse(cursor));
	if (!Number.isSafeInteger(limit) || limit < 1 || limit > 100) {
		throw new RangeError("progression page size is invalid");
	}
	parameters.set("limit", String(limit));
	return parameters.toString();
}

function normalizedConfiguration(value: RewardRuleConfiguration): RewardRuleConfiguration {
	if (!Number.isSafeInteger(value.multiplier_basis_points) ||
		value.multiplier_basis_points < 0 || value.multiplier_basis_points > 100_000 ||
		!Number.isSafeInteger(value.flat_bonus) ||
		value.flat_bonus < -1_000_000_000 || value.flat_bonus > 1_000_000_000) {
		throw new TypeError("reward rule configuration is invalid");
	}
	return value;
}

export const progressionService = {
	listLedger(userID: string, cursor?: string, signal?: AbortSignal): Promise<ProgressionLedgerPage> {
		return api<ProgressionLedgerPage, never>(
			ADMIN_CULTIVATION_API.LEDGER(ledgerQuery(userID, cursor)),
			{ method: "GET", signal, schema: progressionLedgerPageSchema },
		);
	},

	previewAdjustment(input: {
		user_id: string;
		delta: number;
		reversed_entry_id?: string;
	}): Promise<ProgressionAdjustmentPreview> {
		const body = {
			user_id: entityIDSchema.parse(input.user_id),
			delta: input.delta,
			...(input.reversed_entry_id
				? { reversed_entry_id: entityIDSchema.parse(input.reversed_entry_id) }
				: {}),
		};
		const endpoint = body.reversed_entry_id
			? ADMIN_CULTIVATION_API.REVERSAL_PREVIEW
			: ADMIN_CULTIVATION_API.ADJUSTMENT_PREVIEW;
		return api<ProgressionAdjustmentPreview, typeof body>(endpoint, {
			method: "POST", body, schema: progressionAdjustmentPreviewSchema,
		});
	},

	async applyAdjustment(input: ApplyProgressionAdjustmentInput): Promise<ProgressionAdjustmentResult> {
		const body = { ...input, user_id: entityIDSchema.parse(input.user_id) };
		const namespace = body.reversed_entry_id ? "progression-reversal" : "progression-adjustment";
		const purpose = await commandPurpose(namespace, body);
		const endpoint = body.reversed_entry_id
			? ADMIN_CULTIVATION_API.REVERSAL_APPLY
			: ADMIN_CULTIVATION_API.ADJUSTMENT_APPLY;
		return runIdempotentCommand(purpose, (idempotencyKey) =>
			api<ProgressionAdjustmentResult, typeof body>(endpoint, {
				method: "POST", body, idempotencyKey,
				schema: progressionAdjustmentResultSchema,
			}),
		);
	},

	listRewardRules(cursor?: number, signal?: AbortSignal): Promise<RewardRulePage> {
		const parameters = new URLSearchParams({ limit: "25" });
		if (cursor) parameters.set("cursor", String(cursor));
		return api<RewardRulePage, never>(
			ADMIN_CULTIVATION_API.REWARD_RULES(parameters.toString()),
			{ method: "GET", signal, schema: rewardRulePageSchema },
		);
	},

	previewRewardRule(configuration: RewardRuleConfiguration): Promise<RewardRulePublicationPreview> {
		const body = { configuration: normalizedConfiguration(configuration) };
		return api<RewardRulePublicationPreview, typeof body>(
			ADMIN_CULTIVATION_API.REWARD_RULE_PREVIEW,
			{ method: "POST", body, schema: rewardRulePreviewSchema },
		);
	},

	async publishRewardRule(input: PublishRewardRuleInput): Promise<RewardRule> {
		const body = { ...input, configuration: normalizedConfiguration(input.configuration) };
		const purpose = await commandPurpose("reward-rule-publication", body);
		return runIdempotentCommand(purpose, (idempotencyKey) =>
			api<RewardRule, typeof body>(ADMIN_CULTIVATION_API.REWARD_RULE_PUBLISH, {
				method: "POST", body, idempotencyKey, schema: rewardRuleSchema,
			}),
		);
	},

	rankingReconciliationStatus(signal?: AbortSignal): Promise<RankingReconciliationStatus> {
		return api<RankingReconciliationStatus, never>(
			ADMIN_CULTIVATION_API.RANKING_RECONCILIATION,
			{ method: "GET", signal, schema: rankingReconciliationStatusSchema },
		);
	},

	rankingReconciliationPreview(): Promise<RankingReconciliationPreview> {
		return api<RankingReconciliationPreview, Record<string, never>>(
			ADMIN_CULTIVATION_API.RANKING_RECONCILIATION_PREVIEW,
			{ method: "POST", body: {}, schema: rankingReconciliationPreviewSchema },
		);
	},

	async startRankingReconciliation(
		reason: string,
		preview: RankingReconciliationPreview,
	): Promise<RankingReconciliationStart> {
		const confirmed = rankingReconciliationPreviewSchema.parse(preview);
		const body = {
			reason: reason.trim(),
			source_cutoff: confirmed.source_cutoff,
			expected_revision: confirmed.projection_revision,
			source_count: confirmed.source_count,
			repair_count: confirmed.repair_count,
			preview_checksum: confirmed.preview_checksum,
		};
		if (body.reason.length < 3 || body.reason.length > 1024) {
			throw new TypeError("ranking reconciliation reason is invalid");
		}
		const purpose = await commandPurpose("ranking-reconciliation", body);
		return runIdempotentCommand(purpose, (idempotencyKey) =>
			api<RankingReconciliationStart, typeof body>(
				ADMIN_CULTIVATION_API.RANKING_RECONCILIATION,
				{
					method: "POST",
					body,
					idempotencyKey,
					schema: rankingReconciliationStartSchema,
				},
			),
		);
	},
};
