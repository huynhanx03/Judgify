export const CONTEST_CONTENT_LIMITS = {
  ANNOUNCEMENT_TITLE_BYTES: 200,
  ANNOUNCEMENT_MARKDOWN_BYTES: 64 * 1024,
  CLARIFICATION_QUESTION_BYTES: 16 * 1024,
  CLARIFICATION_SUMMARY_BYTES: 16 * 1024,
  CLARIFICATION_ANSWER_BYTES: 32 * 1024,
  REASON_BYTES: 1024,
} as const;

export const CONTEST_ANNOUNCEMENT_STATUS = [
  "draft",
  "published",
  "withdrawn",
] as const;

export const CONTEST_CLARIFICATION_STATUS = [
  "open",
  "answered",
  "closed",
  "withdrawn",
] as const;
