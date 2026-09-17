import {
  NOTIFICATION_CATEGORIES,
  NOTIFICATION_CONTENT_KINDS,
  NOTIFICATION_POLICY,
  REQUIRED_NOTIFICATION_CATEGORIES,
  type NotificationCategory,
  type NotificationContentKind,
} from "@/constants/notification";
import {
  compareISODateTime,
  cursorSchema,
  entityIDSchema,
  enumSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import {
  arraySchema,
  booleanSchema,
  integerSchema,
  optionalSchema,
  recordAt,
  strictObjectSchema,
  stringSchema,
  type Schema,
} from "@/lib/api/schema";
import type {
  Notification,
  NotificationCampaignContent,
  NotificationListResponse,
  NotificationLivePayload,
  NotificationMutationResponse,
  NotificationParameter,
  NotificationParameters,
  NotificationPreference,
  NotificationUnreadResponse,
} from "@/types/notification";

const notificationCategorySchema = enumSchema(
  NOTIFICATION_CATEGORIES,
) as Schema<NotificationCategory>;
const notificationContentKindSchema = enumSchema(
  NOTIFICATION_CONTENT_KINDS,
) as Schema<NotificationContentKind>;
const requiredCategories = new Set<NotificationCategory>(
  REQUIRED_NOTIFICATION_CATEGORIES,
);
const notificationTypeSchema = stringSchema({
  minimumLength: 1,
  maximumLength: NOTIFICATION_POLICY.MAXIMUM_TYPE_LENGTH,
  pattern: /^[a-z][a-z0-9_]{0,31}(?:\.[a-z][a-z0-9_]{0,31}){1,5}\.v[1-9][0-9]{0,8}$/,
  label: "notification type",
});
const messageKeySchema = stringSchema({
  minimumLength: 1,
  maximumLength: NOTIFICATION_POLICY.MAXIMUM_MESSAGE_KEY_LENGTH,
  pattern: /^[a-z][a-z0-9_]{0,31}(?:\.[a-z][a-z0-9_]{0,31}){1,7}$/,
  label: "notification message key",
});
const actionPathSchema: Schema<string> = {
  parse(value: unknown, path = "$"): string {
    const actionPath = stringSchema({
      minimumLength: 1,
      maximumLength: NOTIFICATION_POLICY.MAXIMUM_ACTION_PATH_LENGTH,
      label: "notification action path",
    }).parse(value, path);
    if (!isSafeNotificationActionPath(actionPath)) {
      throw new TypeError(`${path}: unsafe notification action path`);
    }
    return actionPath;
  },
};

const parameterSchema: Schema<NotificationParameter> = {
  parse(value: unknown, path = "$"): NotificationParameter {
    if (
      value === null ||
      typeof value === "string" ||
      typeof value === "boolean"
    ) {
      return value;
    }
    if (typeof value === "number" && Number.isFinite(value)) return value;
    throw new TypeError(`${path}: invalid notification parameter`);
  },
};

const notificationParametersSchema: Schema<NotificationParameters> = {
  parse(value: unknown, path = "$"): NotificationParameters {
    const params = recordAt(value, path);
    const entries = Object.entries(params);
    if (entries.length > 64) {
      throw new TypeError(`${path}: too many notification parameters`);
    }
    const output: Record<string, NotificationParameter> = Object.create(null);
    for (const [key, parameter] of entries) {
      if (!/^[A-Za-z][A-Za-z0-9_]{0,63}$/.test(key)) {
        throw new TypeError(`${path}.${key}: invalid notification parameter name`);
      }
      output[key] = parameterSchema.parse(parameter, `${path}.${key}`);
    }
    return output;
  },
};

const rawNotificationSchema = strictObjectSchema({
  id: entityIDSchema,
  type: notificationTypeSchema,
  category: notificationCategorySchema,
  content_kind: notificationContentKindSchema,
  title_key: optionalSchema(messageKeySchema),
  body_key: optionalSchema(messageKeySchema),
  params: optionalSchema(notificationParametersSchema),
  campaign_revision_id: optionalSchema(entityIDSchema),
  campaign_title: optionalSchema(
    stringSchema({
      minimumLength: 1,
      maximumLength: 160,
      label: "notification campaign title",
    }),
  ),
  action_path: optionalSchema(actionPathSchema),
  version: integerSchema({ minimum: 1, label: "notification version" }),
  created_at: isoDateTimeSchema,
  read_at: optionalSchema(isoDateTimeSchema),
  archived_at: optionalSchema(isoDateTimeSchema),
  expires_at: optionalSchema(isoDateTimeSchema),
});

export const notificationSchema: Schema<Notification> = {
  parse(value: unknown, path = "$"): Notification {
    const notification = rawNotificationSchema.parse(value, path);
    const isCatalog = notification.content_kind === "catalog";
    if (
      (isCatalog &&
        (!notification.title_key ||
          !notification.body_key ||
          notification.campaign_revision_id !== undefined ||
          notification.campaign_title !== undefined)) ||
      (!isCatalog &&
        (!notification.campaign_revision_id ||
          notification.title_key !== undefined ||
          notification.body_key !== undefined ||
          notification.params !== undefined))
    ) {
      throw new TypeError(`${path}: inconsistent notification content`);
    }
    if (
      notification.expires_at &&
      compareISODateTime(notification.expires_at, notification.created_at) <= 0
    ) {
      throw new TypeError(`${path}: notification expiry is not after creation`);
    }
    return notification;
  },
};

export const notificationCampaignContentSchema: Schema<NotificationCampaignContent> =
  strictObjectSchema({
    notification_id: entityIDSchema,
    revision_id: entityIDSchema,
    title: stringSchema({
      minimumLength: 1,
      maximumLength: 160,
      label: "notification campaign title",
    }),
    sanitized_html: stringSchema({
      minimumLength: 1,
      maximumLength: 2 * 1024 * 1024,
      label: "notification campaign rendered content",
    }),
    action_path: optionalSchema(actionPathSchema),
  });

const rawListResponseSchema = strictObjectSchema({
  items: arraySchema(notificationSchema, {
    maximumLength: NOTIFICATION_POLICY.MAXIMUM_PAGE_SIZE,
  }),
  next_cursor: optionalSchema(cursorSchema),
  unread_count: integerSchema({ minimum: 0, label: "notification unread count" }),
  inbox_version: integerSchema({ minimum: 0, label: "notification inbox version" }),
});

export const notificationListResponseSchema: Schema<NotificationListResponse> = {
  parse(value: unknown, path = "$"): NotificationListResponse {
    const page = rawListResponseSchema.parse(value, path);
    const seen = new Set<string>();
    for (let index = 0; index < page.items.length; index += 1) {
      const current = page.items[index];
      if (seen.has(current.id)) {
        throw new TypeError(`${path}: duplicate notification`);
      }
      seen.add(current.id);
      const previous = page.items[index - 1];
      if (!previous) continue;
      const temporalOrder = compareISODateTime(
        previous.created_at,
        current.created_at,
      );
      if (temporalOrder < 0 || (temporalOrder === 0 && previous.id < current.id)) {
        throw new TypeError(`${path}: notification page is not stably ordered`);
      }
    }
    if (page.next_cursor && page.items.length === 0) {
      throw new TypeError(`${path}: empty notification page has a cursor`);
    }
    return page;
  },
};

export const notificationUnreadResponseSchema: Schema<NotificationUnreadResponse> =
  strictObjectSchema({
    count: integerSchema({ minimum: 0, label: "notification unread count" }),
    version: integerSchema({ minimum: 0, label: "notification inbox version" }),
  });

const rawMutationResponseSchema = strictObjectSchema({
  notification: optionalSchema(notificationSchema),
  unread_count: integerSchema({ minimum: 0, label: "notification unread count" }),
  inbox_version: integerSchema({ minimum: 0, label: "notification inbox version" }),
  changed: booleanSchema,
});

export const notificationMutationResponseSchema: Schema<NotificationMutationResponse> = {
  parse(value: unknown, path = "$"): NotificationMutationResponse {
    return rawMutationResponseSchema.parse(value, path);
  },
};

const rawPreferenceSchema = strictObjectSchema({
  category: notificationCategorySchema,
  in_app: booleanSchema,
  required: booleanSchema,
  version: integerSchema({ minimum: 0, label: "notification preference version" }),
});

export const notificationPreferenceSchema: Schema<NotificationPreference> = {
  parse(value: unknown, path = "$"): NotificationPreference {
    const preference = rawPreferenceSchema.parse(value, path);
    if (
      preference.required !== requiredCategories.has(preference.category) ||
      (preference.required && !preference.in_app)
    ) {
      throw new TypeError(`${path}: inconsistent notification preference policy`);
    }
    return preference;
  },
};

export const notificationPreferenceListSchema = arraySchema(
  notificationPreferenceSchema,
  { maximumLength: NOTIFICATION_CATEGORIES.length },
);

const rawLivePayloadSchema = strictObjectSchema({
  user_id: entityIDSchema,
  notification: optionalSchema(notificationSchema),
  unread_count: integerSchema({ minimum: 0, label: "notification unread count" }),
  inbox_version: integerSchema({ minimum: 1, label: "notification inbox version" }),
});

export const notificationLivePayloadSchema: Schema<NotificationLivePayload> = {
  parse(value: unknown, path = "$"): NotificationLivePayload {
    return rawLivePayloadSchema.parse(value, path);
  },
};

const UUID_SEGMENT = "[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}";
const SAFE_ACTION_PATH_PATTERN = new RegExp(
  `^(?:/$|/notifications$|/profile(?:/security)?$|/admin(?:/(?:audit|judge|submissions))?$|/(?:contests|materials|problems|submissions)/${UUID_SEGMENT}$|/admin/(?:submissions|users)/${UUID_SEGMENT}$)`,
);

/** Defense in depth for paths already validated by the API. */
export function isSafeNotificationActionPath(value: string): boolean {
  if (
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\") ||
    value.includes("?") ||
    value.includes("#") ||
    /[\u0000-\u001f\u007f]/.test(value)
  ) {
    return false;
  }
  return SAFE_ACTION_PATH_PATTERN.test(value);
}

/** Parses only the browser-safe live projection, never the full outbox payload. */
export function parseNotificationLivePayload(
  data: unknown,
  aggregateVersion: number | undefined,
  path = "$.data",
): NotificationLivePayload {
  const payload = notificationLivePayloadSchema.parse(data, path);
  if (aggregateVersion !== payload.inbox_version) {
    throw new TypeError(`${path}: live inbox version mismatch`);
  }
  return payload;
}
