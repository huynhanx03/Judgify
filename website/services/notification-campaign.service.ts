import { NOTIFICATION_CAMPAIGN_API } from "@/constants/api/notification-campaign";
import {
  CAMPAIGN_AUDIENCES,
  CAMPAIGN_LIMITS,
  CAMPAIGN_RECIPIENT_STATUSES,
  CAMPAIGN_STATUSES,
} from "@/constants/notification-campaign";
import { api } from "@/lib/api/client";
import { commandPurpose } from "@/lib/api/command-purpose";
import { entityIDSchema, isoDateTimeSchema } from "@/lib/api/contracts";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import {
  campaignDryRunSchema,
  campaignCancelResultSchema,
  campaignPageSchema,
  campaignRecipientPageSchema,
  campaignScheduleResultSchema,
  campaignSchema,
} from "@/lib/notifications/campaign-schema";
import type {
  CampaignAudienceInput,
  CampaignDryRun,
  CampaignCancelResult,
  CampaignListQuery,
  CampaignPage,
  CampaignRecipientListQuery,
  CampaignRecipientPage,
  CampaignScheduleResult,
  CancelCampaignInput,
  CreateCampaignInput,
  DryRunCampaignInput,
  NotificationCampaign,
  ReviseCampaignInput,
  ScheduleCampaignInput,
} from "@/types/notification-campaign";

const encoder = new TextEncoder();

function boundedText(
  value: string,
  maximumBytes: number,
  label: string,
): string {
  const normalized = value.trim();
  if (
    !normalized ||
    encoder.encode(normalized).byteLength > maximumBytes ||
    /[\u0000]/.test(normalized)
  ) {
    throw new TypeError(`${label} is outside the supported boundary`);
  }
  return normalized;
}

function optionalBoundedText(
  value: string | undefined,
  maximumBytes: number,
): string | undefined {
  const normalized = value?.trim();
  if (!normalized) return undefined;
  if (
    encoder.encode(normalized).byteLength > maximumBytes ||
    /[\u0000-\u001f\u007f]/.test(normalized)
  ) {
    throw new TypeError("optional campaign field is outside the boundary");
  }
  return normalized;
}

function normalizeAudience(
  value: CampaignAudienceInput,
): CampaignAudienceInput {
  if (!CAMPAIGN_AUDIENCES.includes(value.kind)) {
    throw new TypeError("unsupported campaign audience");
  }
  const roleID = value.role_id
    ? entityIDSchema.parse(value.role_id)
    : undefined;
  const userIDs = Array.from(
    new Set((value.user_ids ?? []).map((id) => entityIDSchema.parse(id))),
  ).sort();
  if (
    (value.kind === "explicit_users" &&
      (!userIDs.length || userIDs.length > CAMPAIGN_LIMITS.EXPLICIT_USERS || roleID)) ||
    (value.kind === "role" && (!roleID || userIDs.length > 0)) ||
    (value.kind === "all_active" && (roleID || userIDs.length > 0))
  ) {
    throw new TypeError("campaign audience shape is invalid");
  }
  return {
    kind: value.kind,
    ...(roleID ? { role_id: roleID } : {}),
    ...(userIDs.length ? { user_ids: userIDs } : {}),
  };
}

function normalizeContent<T extends CreateCampaignInput>(
  value: T,
): T {
  return {
    ...value,
    title: boundedText(
      value.title,
      CAMPAIGN_LIMITS.TITLE_BYTES,
      "campaign title",
    ),
    body_markdown: boundedText(
      value.body_markdown,
      CAMPAIGN_LIMITS.BODY_BYTES,
      "campaign body",
    ),
    action_path: optionalBoundedText(
      value.action_path,
      CAMPAIGN_LIMITS.ACTION_PATH_BYTES,
    ),
    reason: boundedText(
      value.reason,
      CAMPAIGN_LIMITS.REASON_BYTES,
      "campaign reason",
    ),
    audience: normalizeAudience(value.audience),
  };
}

function campaignQuery(query: CampaignListQuery): string {
  const parameters = new URLSearchParams();
  const limit = query.limit ?? CAMPAIGN_LIMITS.PAGE_SIZE;
  if (
    !Number.isSafeInteger(limit) ||
    limit < 1 ||
    limit > CAMPAIGN_LIMITS.MAXIMUM_PAGE_SIZE
  ) {
    throw new RangeError("campaign page size is invalid");
  }
  parameters.set("limit", String(limit));
  if (query.status) {
    if (!CAMPAIGN_STATUSES.includes(query.status)) {
      throw new TypeError("campaign status is invalid");
    }
    parameters.set("status", query.status);
  }
  if (query.cursor) {
    if (query.cursor.length > CAMPAIGN_LIMITS.CURSOR_LENGTH) {
      throw new TypeError("campaign cursor is invalid");
    }
    parameters.set("cursor", query.cursor);
  }
  return parameters.toString();
}

function recipientQuery(query: CampaignRecipientListQuery): string {
  const parameters = new URLSearchParams();
  const limit = query.limit ?? CAMPAIGN_LIMITS.PAGE_SIZE;
  if (
    !Number.isSafeInteger(limit) ||
    limit < 1 ||
    limit > CAMPAIGN_LIMITS.MAXIMUM_PAGE_SIZE
  ) {
    throw new RangeError("campaign recipient page size is invalid");
  }
  parameters.set("limit", String(limit));
  if (query.status) {
    if (!CAMPAIGN_RECIPIENT_STATUSES.includes(query.status)) {
      throw new TypeError("campaign recipient status is invalid");
    }
    parameters.set("status", query.status);
  }
  if (query.cursor) {
    if (query.cursor.length > CAMPAIGN_LIMITS.CURSOR_LENGTH) {
      throw new TypeError("campaign recipient cursor is invalid");
    }
    parameters.set("cursor", query.cursor);
  }
  return parameters.toString();
}

export const notificationCampaignService = {
  async list(
    query: CampaignListQuery = {},
    signal?: AbortSignal,
  ): Promise<CampaignPage> {
    const page = await api<CampaignPage, never>(
      NOTIFICATION_CAMPAIGN_API.LIST(campaignQuery(query)),
      { method: "GET", signal, schema: campaignPageSchema },
    );
    if (page.next_cursor && page.next_cursor === query.cursor) {
      throw new TypeError("campaign cursor did not advance");
    }
    return page;
  },

  get(id: string, signal?: AbortSignal): Promise<NotificationCampaign> {
    const campaignID = entityIDSchema.parse(id);
    return api<NotificationCampaign, never>(
      NOTIFICATION_CAMPAIGN_API.GET(campaignID),
      { method: "GET", signal, schema: campaignSchema },
    );
  },

  create(input: CreateCampaignInput): Promise<NotificationCampaign> {
    const body = normalizeContent(input);
    return api<NotificationCampaign, CreateCampaignInput>(
      NOTIFICATION_CAMPAIGN_API.CREATE,
      {
        method: "POST",
        body,
        idempotencyKey: crypto.randomUUID(),
        schema: campaignSchema,
      },
    );
  },

  revise(
    id: string,
    input: ReviseCampaignInput,
  ): Promise<NotificationCampaign> {
    const campaignID = entityIDSchema.parse(id);
    if (!Number.isSafeInteger(input.expected_version) || input.expected_version < 1) {
      throw new TypeError("campaign version is invalid");
    }
    const body = normalizeContent(input);
    return api<NotificationCampaign, ReviseCampaignInput>(
      NOTIFICATION_CAMPAIGN_API.REVISE(campaignID),
      {
        method: "PUT",
        body,
        idempotencyKey: crypto.randomUUID(),
        schema: campaignSchema,
      },
    );
  },

  dryRun(
    id: string,
    input: DryRunCampaignInput,
  ): Promise<CampaignDryRun> {
    const campaignID = entityIDSchema.parse(id);
    if (!Number.isSafeInteger(input.expected_version) || input.expected_version < 1) {
      throw new TypeError("campaign version is invalid");
    }
    const body: DryRunCampaignInput = {
      expected_version: input.expected_version,
      ...(input.scheduled_for
        ? { scheduled_for: isoDateTimeSchema.parse(input.scheduled_for) }
        : {}),
    };
    return api<CampaignDryRun, DryRunCampaignInput>(
      NOTIFICATION_CAMPAIGN_API.DRY_RUN(campaignID),
      {
        method: "POST",
        body,
        idempotencyKey: crypto.randomUUID(),
        schema: campaignDryRunSchema,
      },
    );
  },

  async schedule(
    id: string,
    input: ScheduleCampaignInput,
  ): Promise<CampaignScheduleResult> {
    const campaignID = entityIDSchema.parse(id);
    const body = {
      confirmation_token: boundedText(
        input.confirmation_token,
        128,
        "confirmation token",
      ),
      reason: boundedText(
        input.reason,
        CAMPAIGN_LIMITS.REASON_BYTES,
        "campaign reason",
      ),
    };
    const purpose = await commandPurpose("campaign-schedule", {
      campaign_id: campaignID,
      confirmation_token: body.confirmation_token,
      reason: body.reason,
    });
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api<CampaignScheduleResult, ScheduleCampaignInput>(
        NOTIFICATION_CAMPAIGN_API.SCHEDULE(campaignID),
        {
          method: "POST",
          body,
          idempotencyKey,
          schema: campaignScheduleResultSchema,
        },
      ),
    );
  },

  async cancel(
    id: string,
    input: CancelCampaignInput,
  ): Promise<CampaignCancelResult> {
    const campaignID = entityIDSchema.parse(id);
    if (
      !Number.isSafeInteger(input.expected_campaign_version) ||
      input.expected_campaign_version < 1 ||
      !Number.isSafeInteger(input.expected_operation_version) ||
      input.expected_operation_version < 1
    ) {
      throw new TypeError("campaign cancellation version is invalid");
    }
    const body: CancelCampaignInput = {
      expected_campaign_version: input.expected_campaign_version,
      expected_operation_version: input.expected_operation_version,
      reason: boundedText(
        input.reason,
        CAMPAIGN_LIMITS.REASON_BYTES,
        "campaign cancellation reason",
      ),
    };
    const purpose = await commandPurpose("campaign-cancel", {
      campaign_id: campaignID,
      ...body,
    });
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api<CampaignCancelResult, CancelCampaignInput>(
        NOTIFICATION_CAMPAIGN_API.CANCEL(campaignID),
        {
          method: "POST",
          body,
          idempotencyKey,
          schema: campaignCancelResultSchema,
        },
      ),
    );
  },

  async recipients(
    id: string,
    query: CampaignRecipientListQuery = {},
    signal?: AbortSignal,
  ): Promise<CampaignRecipientPage> {
    const campaignID = entityIDSchema.parse(id);
    const page = await api<CampaignRecipientPage, never>(
      NOTIFICATION_CAMPAIGN_API.RECIPIENTS(
        campaignID,
        recipientQuery(query),
      ),
      { method: "GET", signal, schema: campaignRecipientPageSchema },
    );
    if (page.next_cursor && page.next_cursor === query.cursor) {
      throw new TypeError("campaign recipient cursor did not advance");
    }
    return page;
  },
};
