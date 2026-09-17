import type { EntityID, ISODateTime } from "@/types/api";
import type { Operation } from "@/types/operation";

export interface ProgressionLedgerEntry {
	id: EntityID;
	kind: string;
	source_type: string;
	source_aggregate_id: EntityID;
	source_version: number;
	reversed_entry_id?: EntityID;
	/** Immutable inputs selected for solve-derived rewards only. */
	reward_rule_revision_id?: EntityID;
	reward_profile_revision_id?: EntityID;
	/** SHA-256 checksum binding immutable source facts to this ledger vector. */
	source_checksum: string;
	delta: number;
	balance_before: number;
	balance_after: number;
	progression_version: number;
	incident_id?: string;
	reason?: string;
	correlation_id: string;
	occurred_at: ISODateTime;
}

export interface ProgressionLedgerPage {
	entries: ProgressionLedgerEntry[];
	next_cursor?: EntityID;
}

export interface ProgressionAdjustmentPreview {
	user_id: EntityID;
	delta: number;
	balance_before: number;
	balance_after: number;
	expected_version: number;
	reversed_entry_id?: EntityID;
	preview_checksum: string;
}

export interface ApplyProgressionAdjustmentInput {
	user_id: EntityID;
	delta: number;
	expected_balance: number;
	expected_version: number;
	incident_id: string;
	reason: string;
	preview_checksum: string;
	reversed_entry_id?: EntityID;
}

export interface ProgressionAdjustmentResult {
	ledger_entry_id: EntityID;
	user_id: EntityID;
	delta: number;
	balance_before: number;
	balance_after: number;
	version: number;
	replayed?: boolean;
	created_at: ISODateTime;
}

export interface RewardRuleConfiguration {
	multiplier_basis_points: number;
	flat_bonus: number;
}

export interface RewardRulePublicationPreview {
	rule_key: string;
	current_revision: number;
	next_revision: number;
	algorithm_version: string;
	configuration: RewardRuleConfiguration;
	preview_checksum: string;
}

export interface RewardRule {
	id: EntityID;
	rule_key: string;
	revision: number;
	algorithm_version: string;
	configuration: RewardRuleConfiguration;
	checksum: string;
	effective_from: ISODateTime;
	replayed?: boolean;
}

export interface RewardRulePage {
	rules: RewardRule[];
	next_cursor?: number;
}

export interface PublishRewardRuleInput {
	expected_revision: number;
	algorithm_version: string;
	configuration: RewardRuleConfiguration;
	reason: string;
	preview_checksum: string;
}

export interface RankingReconciliationStart {
	operation: Operation;
	source_cutoff: ISODateTime;
	source_count: number;
	repair_count: number;
	projection_revision: number;
	idempotent_replay: boolean;
}

export interface RankingReconciliationPreview {
	source_cutoff: ISODateTime;
	projection_revision: number;
	source_count: number;
	repair_count: number;
	preview_checksum: string;
}

export interface RankingReconciliationStatus {
	projection_revision: number;
	last_reconciled_at?: ISODateTime;
	latest_operation?: Operation;
}
