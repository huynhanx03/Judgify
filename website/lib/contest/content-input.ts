import { CONTEST_CONTENT_LIMITS } from "@/constants/contest-content";
import type { AnswerClarificationInput } from "@/services/contest-content.service";
import type { ContestAnnouncement } from "@/types/contest";

const encoder = new TextEncoder();
const FORBIDDEN_PLAIN_TEXT = /[\p{Cc}\p{Cf}]/u;

export interface AnnouncementEditorValue {
  audience: ContestAnnouncement["audience"];
  title: string;
  markdown: string;
  scheduledForLocal: string;
  reason: string;
}

function validText(
  value: string,
  maximumBytes: number,
  allowMultiline: boolean,
): boolean {
  const normalized = value.trim();
  if (!normalized || normalized !== value || encoder.encode(value).byteLength > maximumBytes) {
    return false;
  }
  for (const character of value) {
    if (
      allowMultiline &&
      (character === "\n" || character === "\r" || character === "\t")
    ) {
      continue;
    }
    if (FORBIDDEN_PLAIN_TEXT.test(character)) return false;
  }
  return allowMultiline || !/[\r\n]/.test(value);
}

export function announcementEditorIsValid(
  value: AnnouncementEditorValue,
): boolean {
  const title = value.title.trim();
  const markdown = value.markdown.trim();
  const reason = value.reason.trim();
  if (
    !["public", "participants", "jury"].includes(value.audience) ||
    !validText(
      title,
      CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_TITLE_BYTES,
      false,
    ) ||
    !validText(
      markdown,
      CONTEST_CONTENT_LIMITS.ANNOUNCEMENT_MARKDOWN_BYTES,
      true,
    ) ||
    !validText(reason, CONTEST_CONTENT_LIMITS.REASON_BYTES, true)
  ) {
    return false;
  }
  if (!value.scheduledForLocal) return true;
  const scheduled = new Date(value.scheduledForLocal);
  return !Number.isNaN(scheduled.getTime()) && scheduled.getTime() > Date.now();
}

export function announcementEditorInput(value: AnnouncementEditorValue) {
  if (!announcementEditorIsValid(value)) {
    throw new TypeError("invalid contest announcement draft");
  }
  return {
    audience: value.audience,
    title: value.title.trim(),
    markdown: value.markdown.trim(),
    ...(value.scheduledForLocal
      ? { scheduled_for: new Date(value.scheduledForLocal).toISOString() }
      : {}),
    reason: value.reason.trim(),
  };
}

export function clarificationAnswerIsValid(
  value: AnswerClarificationInput,
): boolean {
  return (
    ["requester", "participants"].includes(value.audience) &&
    validText(
      value.answer_markdown,
      CONTEST_CONTENT_LIMITS.CLARIFICATION_ANSWER_BYTES,
      true,
    ) &&
    validText(value.reason, CONTEST_CONTENT_LIMITS.REASON_BYTES, true) &&
    (value.audience !== "participants" ||
      (value.public_summary_markdown !== undefined &&
        validText(
          value.public_summary_markdown,
          CONTEST_CONTENT_LIMITS.CLARIFICATION_SUMMARY_BYTES,
          true,
        )))
  );
}

export function clarificationQuestionIsValid(value: string): boolean {
  return validText(
    value.trim(),
    CONTEST_CONTENT_LIMITS.CLARIFICATION_QUESTION_BYTES,
    true,
  );
}

export function contestContentReasonIsValid(value: string): boolean {
  return validText(
    value.trim(),
    CONTEST_CONTENT_LIMITS.REASON_BYTES,
    true,
  );
}

export function announcementScheduleLocal(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

export function utf8Bytes(value: string): number {
  return encoder.encode(value).byteLength;
}
