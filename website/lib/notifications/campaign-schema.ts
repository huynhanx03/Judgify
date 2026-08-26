import {
  arraySchema,
  booleanSchema,
  ContractError,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  cursorSchema,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  CAMPAIGN_AUDIENCES,
  CAMPAIGN_AUDIENCE,
  CAMPAIGN_LIMITS,
  CAMPAIGN_RECIPIENT_STATUSES,
  CAMPAIGN_STATUS,
  CAMPAIGN_STATUSES,
} from "@/constants/notification-campaign";
import {
  OPERATION_KIND_PATTERN,
  OPERATION_REQUESTED_ACTION,
  OPERATION_STATUS,
  OPERATION_STATUSES,
} from "@/constants/operation";
import type {
  CampaignAudience,
  CampaignAudienceMember,
  CampaignDryRun,
  CampaignCancelResult,
  CampaignOperation,
  CampaignPage,
  CampaignRecipient,
  CampaignRecipientCounts,
  CampaignRecipientPage,
  CampaignRevision,
  CampaignScheduleResult,
  NotificationCampaign,
} from "@/types/notification-campaign";

const count = integerSchema({ minimum: 0, label: "campaign count" });
const version = integerSchema({ minimum: 1, label: "campaign version" });
const safeCode = stringSchema({
  minimumLength: 1,
  maximumLength: 64,
  pattern: /^[a-z][a-z0-9_]{0,63}$/,
  label: "safe campaign code",
});

const rawAudienceSchema = strictObjectSchema({
  kind: enumSchema(CAMPAIGN_AUDIENCES),
  role_id: optionalSchema(entityIDSchema),
  explicit_users: optionalSchema(
    arraySchema(entityIDSchema, {
      maximumLength: CAMPAIGN_LIMITS.EXPLICIT_USERS,
      unique: true,
    }),
  ),
});

const campaignAudienceSchema: Schema<CampaignAudience> = {
  parse(value: unknown, path = "$"): CampaignAudience {
    const audience = rawAudienceSchema.parse(value, path);
    const explicit = audience.explicit_users ?? [];
    if (
      (audience.kind === CAMPAIGN_AUDIENCE.EXPLICIT_USERS &&
        (!explicit.length || audience.role_id)) ||
      (audience.kind === CAMPAIGN_AUDIENCE.ROLE &&
        (!audience.role_id || explicit.length > 0)) ||
      (audience.kind === CAMPAIGN_AUDIENCE.ALL_ACTIVE &&
        (audience.role_id || explicit.length > 0))
    ) {
      throw new ContractError("campaign audience shape is inconsistent", path);
    }
    return audience;
  },
};

const campaignRevisionSchema: Schema<CampaignRevision> = strictObjectSchema({
  id: entityIDSchema,
  revision_number: version,
  title: stringSchema({
    minimumLength: 1,
    maximumLength: CAMPAIGN_LIMITS.TITLE_BYTES,
    label: "campaign title",
  }),
  body_markdown: stringSchema({
    minimumLength: 1,
    maximumLength: CAMPAIGN_LIMITS.BODY_BYTES,
    label: "campaign body",
  }),
  sanitized_html: stringSchema({
    minimumLength: 1,
    maximumLength: 512 * 1024,
    label: "campaign preview",
  }),
  action_path: optionalSchema(
    stringSchema({
      minimumLength: 1,
      maximumLength: CAMPAIGN_LIMITS.ACTION_PATH_BYTES,
      label: "campaign action path",
    }),
  ),
  audience: campaignAudienceSchema,
  author_id: entityIDSchema,
  created_at: isoDateTimeSchema,
});

const recipientCountsSchema: Schema<CampaignRecipientCounts> = {
  parse(value: unknown, path = "$"): CampaignRecipientCounts {
    const result = strictObjectSchema({
      total: count,
      pending: count,
      created: count,
      skipped: count,
      failed: count,
    }).parse(value, path);
    if (
      result.total !==
      result.pending + result.created + result.skipped + result.failed
    ) {
      throw new ContractError("campaign recipient totals are inconsistent", path);
    }
    return result;
  },
};

const campaignOperationSchema: Schema<CampaignOperation> = strictObjectSchema({
  id: entityIDSchema,
  status: enumSchema(OPERATION_STATUSES),
  processed: count,
  succeeded: count,
  failed: count,
  total: count,
  version,
  requested_action: enumSchema(Object.values(OPERATION_REQUESTED_ACTION)),
  last_error_code: optionalSchema(
    stringSchema({
      minimumLength: 1,
      maximumLength: 64,
      pattern: OPERATION_KIND_PATTERN,
      label: "operation failure code",
    }),
  ),
});

const rawCampaignSchema = strictObjectSchema({
  id: entityIDSchema,
  status: enumSchema(CAMPAIGN_STATUSES),
  version,
  revision: campaignRevisionSchema,
  recipients: recipientCountsSchema,
  operation: optionalSchema(campaignOperationSchema),
  scheduled_for: optionalSchema(isoDateTimeSchema),
  last_failure_code: optionalSchema(safeCode),
  created_by: entityIDSchema,
  scheduled_by: optionalSchema(entityIDSchema),
  cancelled_by: optionalSchema(entityIDSchema),
  cancel_reason: optionalSchema(
    stringSchema({
      minimumLength: 1,
      maximumLength: CAMPAIGN_LIMITS.REASON_BYTES,
      label: "campaign cancellation reason",
    }),
  ),
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
  scheduled_at: optionalSchema(isoDateTimeSchema),
  started_at: optionalSchema(isoDateTimeSchema),
  completed_at: optionalSchema(isoDateTimeSchema),
  cancelled_at: optionalSchema(isoDateTimeSchema),
});

export const campaignSchema: Schema<NotificationCampaign> = {
  parse(value: unknown, path = "$"): NotificationCampaign {
    const campaign = rawCampaignSchema.parse(value, path);
    if (
      campaign.operation &&
      (campaign.operation.processed >
        campaign.operation.total ||
        campaign.operation.processed !==
          campaign.operation.succeeded + campaign.operation.failed)
    ) {
      throw new ContractError("campaign operation progress is inconsistent", path);
    }
    if (
      campaign.operation &&
      campaign.operation.requested_action !== OPERATION_REQUESTED_ACTION.NONE &&
      campaign.operation.status !== OPERATION_STATUS.RUNNING
    ) {
      throw new ContractError("campaign operation control is inconsistent", path);
    }
    const cancellationFields = [
      campaign.cancelled_by,
      campaign.cancel_reason,
      campaign.cancelled_at,
    ].filter((value) => value !== undefined).length;
    if (
      (cancellationFields !== 0 && cancellationFields !== 3) ||
      (cancellationFields === 3 &&
        campaign.status !== CAMPAIGN_STATUS.CANCELLED)
    ) {
      throw new ContractError("campaign cancellation evidence is inconsistent", path);
    }
    return campaign;
  },
};

const rawCampaignPageSchema: Schema<CampaignPage> = strictObjectSchema({
  items: arraySchema(campaignSchema, {
    maximumLength: CAMPAIGN_LIMITS.MAXIMUM_PAGE_SIZE,
  }),
  next_cursor: optionalSchema(cursorSchema),
});

export const campaignPageSchema: Schema<CampaignPage> = {
  parse(value: unknown, path = "$"): CampaignPage {
    const page = rawCampaignPageSchema.parse(value, path);
    const ids = new Set(page.items.map(({ id }) => id));
    if (ids.size !== page.items.length) {
      throw new ContractError("campaign page contains duplicate IDs", path);
    }
    if (page.next_cursor && page.items.length === 0) {
      throw new ContractError("empty campaign page cannot advance", path);
    }
    return page;
  },
};

const audienceMemberSchema: Schema<CampaignAudienceMember> =
  strictObjectSchema({
    user_id: entityIDSchema,
    username: stringSchema({
      minimumLength: 1,
      maximumLength: 64,
      label: "campaign audience username",
    }),
  });

export const campaignDryRunSchema: Schema<CampaignDryRun> =
  strictObjectSchema({
    campaign_id: entityIDSchema,
    revision_id: entityIDSchema,
    campaign_version: version,
    recipient_count: count,
    sample: arraySchema(audienceMemberSchema, {
      maximumLength: CAMPAIGN_LIMITS.SAMPLE_SIZE,
    }),
    estimated_at: isoDateTimeSchema,
    confirmation_token: stringSchema({
      minimumLength: 32,
      maximumLength: 128,
      pattern: /^[A-Za-z0-9_-]+$/,
      label: "campaign confirmation token",
    }),
    confirmation_expires_at: isoDateTimeSchema,
    scheduled_for: optionalSchema(isoDateTimeSchema),
  });

export const campaignScheduleResultSchema: Schema<CampaignScheduleResult> =
  strictObjectSchema({
    campaign: campaignSchema,
    operation: campaignOperationSchema,
    replayed: booleanSchema,
  });

export const campaignCancelResultSchema: Schema<CampaignCancelResult> =
  strictObjectSchema({
    campaign: campaignSchema,
    operation: campaignOperationSchema,
    replayed: booleanSchema,
  });

const campaignRecipientSchema: Schema<CampaignRecipient> = strictObjectSchema({
  user_id: entityIDSchema,
  status: enumSchema(CAMPAIGN_RECIPIENT_STATUSES),
  notification_id: optionalSchema(entityIDSchema),
  skip_code: optionalSchema(safeCode),
  failure_code: optionalSchema(safeCode),
  attempt_count: count,
  last_attempt_at: optionalSchema(isoDateTimeSchema),
  processed_at: optionalSchema(isoDateTimeSchema),
});

const rawCampaignRecipientPageSchema: Schema<CampaignRecipientPage> =
  strictObjectSchema({
    items: arraySchema(campaignRecipientSchema, {
      maximumLength: CAMPAIGN_LIMITS.MAXIMUM_PAGE_SIZE,
    }),
    next_cursor: optionalSchema(cursorSchema),
  });

export const campaignRecipientPageSchema: Schema<CampaignRecipientPage> = {
  parse(value: unknown, path = "$"): CampaignRecipientPage {
    const page = rawCampaignRecipientPageSchema.parse(value, path);
    const userIDs = new Set(page.items.map(({ user_id }) => user_id));
    if (userIDs.size !== page.items.length) {
      throw new ContractError(
        "campaign recipient page contains duplicate user IDs",
        path,
      );
    }
    if (page.next_cursor && page.items.length === 0) {
      throw new ContractError(
        "empty campaign recipient page cannot advance",
        path,
      );
    }
    return page;
  },
};
