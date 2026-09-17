import type { EntityID, ISODateTime } from "@/types/api";
import type { Operation } from "@/types/operation";

export const ATTRIBUTE_DATA_TYPES = [
  "string",
  "number",
  "boolean",
  "date",
] as const;

export type AttributeDataType = (typeof ATTRIBUTE_DATA_TYPES)[number];
export type AttributeVisibility = "public" | "private" | "admin";

export interface AttributeDefinitionRevision {
  id: EntityID;
  definition_id: EntityID;
  revision: number;
  display_order: number;
  data_type: AttributeDataType;
  label: string;
  description: string;
  visibility: AttributeVisibility;
  user_editable: boolean;
  validation: Record<string, unknown>;
  created_at: ISODateTime;
}

export interface AttributeDefinitionRevisionPage {
  records: AttributeDefinitionRevision[];
  next_cursor?: string;
}

export interface AttributeDefinition {
  id: EntityID;
  key: string;
  kind: "system" | "custom";
  required_on_onboarding: boolean;
  status: "active" | "archived";
  version: number;
  draft_revision_id: EntityID;
  active_revision_id: EntityID;
  draft_revision: AttributeDefinitionRevision;
  active_revision: AttributeDefinitionRevision;
}

export interface CreateAttributeDefinitionInput {
  key: string;
  display_order: number;
  data_type: AttributeDataType;
  label: string;
  description: string;
  visibility: AttributeVisibility;
  user_editable: boolean;
  required_on_onboarding: boolean;
  validation: Record<string, unknown>;
  reason: string;
}

export interface UpdateAttributeDefinitionInput {
  expected_version: number;
  display_order?: number;
  data_type?: AttributeDataType;
  label?: string;
  description?: string;
  visibility?: AttributeVisibility;
  user_editable?: boolean;
  required_on_onboarding?: boolean;
  validation?: Record<string, unknown>;
  reason: string;
}

export type AttributeLifecycleAction = "activate" | "archive";

export interface AttributeLifecycleCommandInput {
  expected_version: number;
  candidate_revision_id?: EntityID;
  reason: string;
}

export interface AttributeLifecyclePreview {
  action: AttributeLifecycleAction;
  definition_id: EntityID;
  expected_version: number;
  resulting_version: number;
  current_revision?: AttributeDefinitionRevision;
  candidate_revision?: AttributeDefinitionRevision;
  source_value_count: number;
  target_value_count: number;
  requires_migration: boolean;
}

export interface AttributeLifecyclePreviewResponse {
  preview: AttributeLifecyclePreview;
  confirmation_token: string;
  review_hash: string;
  expires_at: ISODateTime;
}

export interface ReviewedAttributeLifecycleCommand {
  definition_id: EntityID;
  command_id: EntityID;
  input: AttributeLifecycleCommandInput;
  review: AttributeLifecyclePreviewResponse;
}

interface AttributeLifecycleReceiptBase {
  command_id: EntityID;
  definition_id: EntityID;
  version: number;
  definition: AttributeDefinition;
  recorded_at: ISODateTime;
  idempotent_replay: boolean;
}

export interface AttributeLifecycleMigrationScheduledReceipt
  extends AttributeLifecycleReceiptBase {
  status: "migration_scheduled";
  action: "activate";
  event_id?: never;
  active_revision: AttributeDefinitionRevision;
  operation: Operation;
}

export interface AttributeLifecycleCommittedActivationReceipt
  extends AttributeLifecycleReceiptBase {
  status: "committed";
  action: "activate";
  event_id: EntityID;
  active_revision: AttributeDefinitionRevision;
  operation?: never;
}

export interface AttributeLifecycleCommittedArchiveReceipt
  extends AttributeLifecycleReceiptBase {
  status: "committed";
  action: "archive";
  event_id: EntityID;
  active_revision?: never;
  operation?: never;
}

export type AttributeLifecycleReceipt =
  | AttributeLifecycleMigrationScheduledReceipt
  | AttributeLifecycleCommittedActivationReceipt
  | AttributeLifecycleCommittedArchiveReceipt;
