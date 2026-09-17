import { TEXT } from "@/constants/text";
import { APP_LOCALE } from "@/i18n/locale";

const pad = (value: number) => String(value).padStart(2, "0");

function parseDate(value: string): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

const dateTimeMinuteFormatter = new Intl.DateTimeFormat(APP_LOCALE, {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

const calendarDateFormatter = new Intl.DateTimeFormat(APP_LOCALE, {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  timeZone: "UTC",
});

const numberFormatter = new Intl.NumberFormat(APP_LOCALE);
const numberFormatParts = numberFormatter.formatToParts(1000.1);

/** Locale punctuation for exact-decimal renderers that must not round via Intl. */
export const NUMBER_FORMAT_SEPARATORS = Object.freeze({
  group: numberFormatParts.find((part) => part.type === "group")?.value ?? ",",
  decimal:
    numberFormatParts.find((part) => part.type === "decimal")?.value ?? ".",
});

function parseCalendarDate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;

  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
    ? date
    : null;
}

/** Locale-stable number rendering shared by all product surfaces. */
export function formatNumber(value: number): string {
  return Number.isFinite(value)
    ? numberFormatter.format(value)
    : TEXT.COMMON.UNKNOWN;
}

/** Calendar-date rendering that cannot shift a day across client time zones. */
export function formatCalendarDate(value: string): string {
  const date = parseCalendarDate(value);
  return date ? calendarDateFormatter.format(date) : TEXT.COMMON.UNKNOWN;
}

/** Compact timestamp for dense table cells. */
export function formatTime(dateString: string): string {
  const date = parseDate(dateString);
  if (!date) return TEXT.COMMON.UNKNOWN;
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Local date and minute for schedules and other human-facing timelines. */
export function formatDateTimeMinute(dateString: string): string {
  const date = parseDate(dateString);
  return date
    ? dateTimeMinuteFormatter.format(date)
    : TEXT.COMMON.UNKNOWN;
}

/** Full timestamp for detail views and audit-friendly metadata. */
export function formatDateTime(dateString: string): string {
  const date = parseDate(dateString);
  if (!date) return TEXT.COMMON.UNKNOWN;
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function formatDuration(milliseconds: number): string {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) {
    return TEXT.COMMON.UNKNOWN;
  }
  if (milliseconds < 1_000) {
    return `${milliseconds}${TEXT.COMMON.UNITS.MILLISECONDS_SHORT}`;
  }

  return `${(milliseconds / 1_000).toFixed(1)}${TEXT.COMMON.UNITS.SECONDS_SHORT}`;
}

export function formatDurationMinutes(milliseconds: number): string {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) {
    return TEXT.COMMON.UNKNOWN;
  }
  const totalMinutes = Math.floor(milliseconds / 60_000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours === 0) {
    return `${minutes}${TEXT.COMMON.UNITS.MINUTES_SHORT}`;
  }
  return [
    `${hours}${TEXT.COMMON.UNITS.HOURS_SHORT}`,
    minutes > 0
      ? `${minutes}${TEXT.COMMON.UNITS.MINUTES_SHORT}`
      : "",
  ]
    .filter(Boolean)
    .join(" ");
}

export function formatMemory(kilobytes: number): string {
  if (!Number.isFinite(kilobytes) || kilobytes < 0) {
    return TEXT.COMMON.UNKNOWN;
  }
  if (kilobytes < 1_024) {
    return `${kilobytes}${TEXT.COMMON.UNITS.KILOBYTES_SHORT}`;
  }

  return `${(kilobytes / 1_024).toFixed(1)}${TEXT.COMMON.UNITS.MEGABYTES_SHORT}`;
}
