import { TEXT } from "@/constants/text";
import { APP_LANGUAGE } from "@/i18n/locale";
import type { NotificationCategory } from "@/constants/notification";
import type {
  Notification,
  NotificationParameters,
} from "@/types/notification";

const MAXIMUM_PARAMETER_CHARACTERS = 180;
const EMPTY_PARAMETER: string =
  TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.EMPTY;

type MessageResolver = (params: NotificationParameters) => string;

function parameter(
  params: NotificationParameters,
  key: string,
  fallback = EMPTY_PARAMETER,
): string {
  const value = params[key];
  if (value === null || value === undefined || value === "") return fallback;
  const rendered = String(value);
  return rendered.length <= MAXIMUM_PARAMETER_CHARACTERS
    ? rendered
    : `${rendered.slice(0, MAXIMUM_PARAMETER_CHARACTERS - 1)}…`;
}

const titleResolvers: Readonly<Record<string, MessageResolver>> = {
  "notification.submission.judged.title": () =>
    TEXT.NOTIFICATION.MESSAGE.SUBMISSION_JUDGED_TITLE,
  "notification.submission.system_error.title": () =>
    TEXT.NOTIFICATION.MESSAGE.SUBMISSION_SYSTEM_ERROR_TITLE,
  "notification.contest.registration.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_REGISTRATION_TITLE,
  "notification.contest.started.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_STARTED_TITLE,
  "notification.contest.lifecycle.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_LIFECYCLE_TITLE,
  "notification.contest.announcement.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_ANNOUNCEMENT_TITLE,
  "notification.contest.clarification.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_CLARIFICATION_TITLE,
  "notification.contest.final_rank.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_FINAL_RANK_TITLE,
  "notification.contest.rating.title": () =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_RATING_TITLE,
  "notification.security.account.title": () =>
    TEXT.NOTIFICATION.MESSAGE.SECURITY_ACCOUNT_TITLE,
  "notification.security.session.title": () =>
    TEXT.NOTIFICATION.MESSAGE.SECURITY_SESSION_TITLE,
  "notification.authorization.changed.title": () =>
    TEXT.NOTIFICATION.MESSAGE.AUTHORIZATION_CHANGED_TITLE,
  "notification.operation.alert.title": () =>
    TEXT.NOTIFICATION.MESSAGE.OPERATION_ALERT_TITLE,
  "notification.material.published.title": () =>
    TEXT.NOTIFICATION.MESSAGE.MATERIAL_PUBLISHED_TITLE,
};

const bodyResolvers: Readonly<Record<string, MessageResolver>> = {
  "notification.submission.judged.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.SUBMISSION_JUDGED_BODY(
      parameter(
        params,
        "problem_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.SUBMISSION,
      ),
      parameter(
        params,
        "verdict",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.VERDICT,
      ),
    ),
  "notification.submission.system_error.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.SUBMISSION_SYSTEM_ERROR_BODY(
      parameter(
        params,
        "problem_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.SUBMISSION_REFERENCE,
      ),
    ),
  "notification.contest.registration.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_REGISTRATION_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
      parameter(
        params,
        "status",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.REGISTRATION_STATUS,
      ),
    ),
  "notification.contest.started.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_STARTED_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
    ),
  "notification.contest.lifecycle.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_LIFECYCLE_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
      parameter(
        params,
        "status",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.LIFECYCLE_STATUS,
      ),
    ),
  "notification.contest.announcement.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_ANNOUNCEMENT_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
    ),
  "notification.contest.clarification.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_CLARIFICATION_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST_REFERENCE,
      ),
    ),
  "notification.contest.final_rank.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_FINAL_RANK_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
      parameter(
        params,
        "rank",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.EMPTY,
      ),
    ),
  "notification.contest.rating.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.CONTEST_RATING_BODY(
      parameter(
        params,
        "contest_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.CONTEST,
      ),
      parameter(
        params,
        "rating_delta",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.RATING_DELTA,
      ),
    ),
  "notification.security.account.body": () =>
    TEXT.NOTIFICATION.MESSAGE.SECURITY_ACCOUNT_BODY,
  "notification.security.session.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.SECURITY_SESSION_BODY(
      parameter(
        params,
        "device",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.DEVICE,
      ),
    ),
  "notification.authorization.changed.body": () =>
    TEXT.NOTIFICATION.MESSAGE.AUTHORIZATION_CHANGED_BODY,
  "notification.operation.alert.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.OPERATION_ALERT_BODY(
      parameter(params, "summary", TEXT.NOTIFICATION.MESSAGE.FALLBACK_BODY),
    ),
  "notification.material.published.body": (params) =>
    TEXT.NOTIFICATION.MESSAGE.MATERIAL_PUBLISHED_BODY(
      parameter(
        params,
        "material_title",
        TEXT.NOTIFICATION.MESSAGE.PARAMETER_FALLBACK.MATERIAL,
      ),
    ),
};

export interface NotificationCopy {
  title: string;
  body: string;
}

/** Resolves only product-owned catalog keys; unknown keys never render raw. */
export function notificationCopy(notification: Notification): NotificationCopy {
  if (notification.content_kind === "campaign") {
    return {
      title:
        notification.campaign_title ??
        TEXT.NOTIFICATION.MESSAGE.CAMPAIGN_TITLE,
      body: TEXT.NOTIFICATION.MESSAGE.CAMPAIGN_BODY,
    };
  }
  const params = notification.params ?? {};
  return {
    title:
      (notification.title_key && titleResolvers[notification.title_key]?.(params)) ||
      TEXT.NOTIFICATION.MESSAGE.FALLBACK_TITLE,
    body:
      (notification.body_key && bodyResolvers[notification.body_key]?.(params)) ||
      TEXT.NOTIFICATION.MESSAGE.FALLBACK_BODY,
  };
}

export function notificationCategoryCopy(category: NotificationCategory): {
  label: string;
  description: string;
} {
  let copy: { readonly LABEL: string; readonly DESCRIPTION: string };
  switch (category) {
    case "authorization":
      copy = TEXT.NOTIFICATION.CATEGORY.AUTHORIZATION;
      break;
    case "campaign":
      copy = TEXT.NOTIFICATION.CATEGORY.CAMPAIGN;
      break;
    case "contest":
      copy = TEXT.NOTIFICATION.CATEGORY.CONTEST;
      break;
    case "material":
      copy = TEXT.NOTIFICATION.CATEGORY.MATERIAL;
      break;
    case "operation":
      copy = TEXT.NOTIFICATION.CATEGORY.OPERATION;
      break;
    case "security":
      copy = TEXT.NOTIFICATION.CATEGORY.SECURITY;
      break;
    case "submission":
      copy = TEXT.NOTIFICATION.CATEGORY.SUBMISSION;
      break;
  }
  return { label: copy.LABEL, description: copy.DESCRIPTION };
}

export function formatNotificationAge(createdAt: string, now = Date.now()): string {
  const timestamp = Date.parse(createdAt);
  if (!Number.isFinite(timestamp)) return TEXT.COMMON.UNKNOWN;
  const seconds = Math.round((timestamp - now) / 1000);
  const formatter = new Intl.RelativeTimeFormat(APP_LANGUAGE, {
    numeric: "auto",
  });
  if (Math.abs(seconds) < 60) return formatter.format(seconds, "second");
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return formatter.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, "hour");
  const days = Math.round(hours / 24);
  if (Math.abs(days) < 30) return formatter.format(days, "day");
  const months = Math.round(days / 30);
  if (Math.abs(months) < 12) return formatter.format(months, "month");
  return formatter.format(Math.round(months / 12), "year");
}
