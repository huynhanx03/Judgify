import { NOTIFICATION_API } from "@/constants/api/notification";
import {
  NOTIFICATION_CATEGORIES,
  NOTIFICATION_POLICY,
  type NotificationCategory,
} from "@/constants/notification";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import {
  notificationListResponseSchema,
  notificationCampaignContentSchema,
  notificationMutationResponseSchema,
  notificationPreferenceListSchema,
  notificationPreferenceSchema,
  notificationUnreadResponseSchema,
} from "@/lib/notifications/notification-schema";
import type {
  NotificationListQuery,
  NotificationListResponse,
  NotificationCampaignContent,
  NotificationMutationResponse,
  NotificationPreference,
  NotificationUnreadResponse,
  UpdateNotificationPreferenceRequest,
} from "@/types/notification";

function normalizeLimit(limit: number | undefined): number {
  const value = limit ?? NOTIFICATION_POLICY.DEFAULT_PAGE_SIZE;
  if (
    !Number.isSafeInteger(value) ||
    value < 1 ||
    value > NOTIFICATION_POLICY.MAXIMUM_PAGE_SIZE
  ) {
    throw new RangeError("notification page size is outside the supported boundary");
  }
  return value;
}

function listEndpoint(query: NotificationListQuery): string {
  const params = new URLSearchParams();
  params.set("limit", String(normalizeLimit(query.limit)));
  const cursor = query.cursor?.trim();
  if (cursor) {
    if (cursor.length > NOTIFICATION_POLICY.MAXIMUM_CURSOR_LENGTH) {
      throw new RangeError("notification cursor is outside the supported boundary");
    }
    params.set("cursor", cursor);
  }
  if (query.includeArchived) params.set("include_archived", "true");
  return `${NOTIFICATION_API.LIST}?${params.toString()}`;
}

function normalizeCategory(category: string): NotificationCategory {
  const normalized = category.trim() as NotificationCategory;
  if (!NOTIFICATION_CATEGORIES.includes(normalized)) {
    throw new TypeError("unsupported notification category");
  }
  return normalized;
}

export const notificationService = {
  list(
    query: NotificationListQuery = {},
    signal?: AbortSignal,
  ): Promise<NotificationListResponse> {
    return api<NotificationListResponse, never>(listEndpoint(query), {
      method: "GET",
      signal,
      schema: notificationListResponseSchema,
    });
  },

  unread(signal?: AbortSignal): Promise<NotificationUnreadResponse> {
    return api<NotificationUnreadResponse, never>(NOTIFICATION_API.UNREAD, {
      method: "GET",
      signal,
      schema: notificationUnreadResponseSchema,
    });
  },

  campaignContent(
    id: string,
    signal?: AbortSignal,
  ): Promise<NotificationCampaignContent> {
    const notificationID = entityIDSchema.parse(id.trim());
    return api<NotificationCampaignContent, never>(
      NOTIFICATION_API.CAMPAIGN_CONTENT(notificationID),
      {
        method: "GET",
        signal,
        schema: notificationCampaignContentSchema,
      },
    );
  },

  markRead(id: string): Promise<NotificationMutationResponse> {
    const notificationID = entityIDSchema.parse(id.trim());
    return api<NotificationMutationResponse, never>(
      NOTIFICATION_API.MARK_READ(notificationID),
      {
        method: "POST",
        idempotencyKey: crypto.randomUUID(),
        schema: notificationMutationResponseSchema,
      },
    );
  },

  markAllRead(): Promise<NotificationMutationResponse> {
    return api<NotificationMutationResponse, never>(
      NOTIFICATION_API.MARK_ALL_READ,
      {
        method: "POST",
        idempotencyKey: crypto.randomUUID(),
        schema: notificationMutationResponseSchema,
      },
    );
  },

  archive(id: string): Promise<NotificationMutationResponse> {
    const notificationID = entityIDSchema.parse(id.trim());
    return api<NotificationMutationResponse, never>(
      NOTIFICATION_API.ARCHIVE(notificationID),
      {
        method: "DELETE",
        idempotencyKey: crypto.randomUUID(),
        schema: notificationMutationResponseSchema,
      },
    );
  },

  preferences(signal?: AbortSignal): Promise<NotificationPreference[]> {
    return api<NotificationPreference[], never>(NOTIFICATION_API.PREFERENCES, {
      method: "GET",
      signal,
      schema: notificationPreferenceListSchema,
    });
  },

  updatePreference(
    category: string,
    request: UpdateNotificationPreferenceRequest,
  ): Promise<NotificationPreference> {
    const normalized = normalizeCategory(category);
    if (!Number.isSafeInteger(request.expected_version) || request.expected_version < 0) {
      throw new RangeError("notification preference version is invalid");
    }
    return api<NotificationPreference, UpdateNotificationPreferenceRequest>(
      NOTIFICATION_API.PREFERENCE(normalized),
      {
        method: "PUT",
        body: request,
        idempotencyKey: crypto.randomUUID(),
        schema: notificationPreferenceSchema,
      },
    );
  },
};
