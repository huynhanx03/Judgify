import type { Cursor, EntityID, ISODateTime } from "@/types/api";
import {
  CAMPAIGN_AUDIENCE,
  CAMPAIGN_RECIPIENT_STATUS,
  CAMPAIGN_STATUS,
} from "@/constants/notification-campaign";
import type { OperationRequestedAction } from "@/types/operation";

export type CampaignStatus =
  (typeof CAMPAIGN_STATUS)[keyof typeof CAMPAIGN_STATUS];
export type CampaignAudienceKind =
  (typeof CAMPAIGN_AUDIENCE)[keyof typeof CAMPAIGN_AUDIENCE];
export type CampaignRecipientStatus =
  (typeof CAMPAIGN_RECIPIENT_STATUS)[keyof typeof CAMPAIGN_RECIPIENT_STATUS];

export interface CampaignAudience {
  kind: CampaignAudienceKind;
  role_id?: EntityID;
  explicit_users?: EntityID[];
}

export interface CampaignRevision {
  id: EntityID;
  revision_number: number;
  title: string;
  body_markdown: string;
  sanitized_html: string;
  action_path?: string;
  audience: CampaignAudience;
  author_id: EntityID;
  created_at: ISODateTime;
}

export interface CampaignRecipientCounts {
  total: number;
  pending: number;
  created: number;
  skipped: number;
  failed: number;
}

export interface CampaignOperation {
  id: EntityID;
  status: string;
  processed: number;
  succeeded: number;
  failed: number;
  total: number;
  version: number;
  requested_action: OperationRequestedAction;
  last_error_code?: string;
}

export interface NotificationCampaign {
  id: EntityID;
  status: CampaignStatus;
  version: number;
  revision: CampaignRevision;
  recipients: CampaignRecipientCounts;
  operation?: CampaignOperation;
  scheduled_for?: ISODateTime;
  last_failure_code?: string;
  created_by: EntityID;
  scheduled_by?: EntityID;
  cancelled_by?: EntityID;
  cancel_reason?: string;
  created_at: ISODateTime;
  updated_at: ISODateTime;
  scheduled_at?: ISODateTime;
  started_at?: ISODateTime;
  completed_at?: ISODateTime;
  cancelled_at?: ISODateTime;
}

export interface CampaignPage {
  items: NotificationCampaign[];
  next_cursor?: Cursor;
}

export interface CampaignAudienceInput {
  kind: CampaignAudienceKind;
  role_id?: EntityID;
  user_ids?: EntityID[];
}

export interface CreateCampaignInput {
  title: string;
  body_markdown: string;
  action_path?: string;
  audience: CampaignAudienceInput;
  reason: string;
}

export interface ReviseCampaignInput extends CreateCampaignInput {
  expected_version: number;
}

export interface CampaignListQuery {
  status?: CampaignStatus;
  cursor?: Cursor;
  limit?: number;
}

export interface CampaignAudienceMember {
  user_id: EntityID;
  username: string;
}

export interface CampaignDryRun {
  campaign_id: EntityID;
  revision_id: EntityID;
  campaign_version: number;
  recipient_count: number;
  sample: CampaignAudienceMember[];
  estimated_at: ISODateTime;
  confirmation_token: string;
  confirmation_expires_at: ISODateTime;
  scheduled_for?: ISODateTime;
}

export interface DryRunCampaignInput {
  expected_version: number;
  scheduled_for?: string;
}

export interface ScheduleCampaignInput {
  confirmation_token: string;
  reason: string;
}

export interface CampaignScheduleResult {
  campaign: NotificationCampaign;
  operation: CampaignOperation;
  replayed: boolean;
}

export interface CancelCampaignInput {
  expected_campaign_version: number;
  expected_operation_version: number;
  reason: string;
}

export interface CampaignCancelResult {
  campaign: NotificationCampaign;
  operation: CampaignOperation;
  replayed: boolean;
}

export interface CampaignRecipient {
  user_id: EntityID;
  status: CampaignRecipientStatus;
  notification_id?: EntityID;
  skip_code?: string;
  failure_code?: string;
  attempt_count: number;
  last_attempt_at?: ISODateTime;
  processed_at?: ISODateTime;
}

export interface CampaignRecipientPage {
  items: CampaignRecipient[];
  next_cursor?: Cursor;
}

export interface CampaignRecipientListQuery {
  status?: CampaignRecipientStatus;
  cursor?: Cursor;
  limit?: number;
}
