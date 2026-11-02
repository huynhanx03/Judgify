import assert from "node:assert/strict";
import { test } from "vitest";

import {
  announcementEditorInput,
  announcementEditorIsValid,
  announcementScheduleLocal,
  clarificationAnswerIsValid,
  clarificationQuestionIsValid,
  contestContentReasonIsValid,
  utf8Bytes,
} from "@/lib/contest/content-input";

const draft = {
  audience: "public" as const,
  title: "Release notice",
  markdown: "Contest starts tomorrow.",
  scheduledForLocal: "",
  reason: "Correct schedule",
};

test("normalizes a valid announcement exactly at the command boundary", () => {
  assert.deepEqual(announcementEditorInput({ ...draft, title: "  Release notice  " }), {
    audience: "public",
    title: "Release notice",
    markdown: "Contest starts tomorrow.",
    reason: "Correct schedule",
  });
});

test("announcement validation rejects unsafe text, invalid audience, and past schedules", () => {
  assert.equal(announcementEditorIsValid({ ...draft, audience: "other" as never }), false);
  assert.equal(announcementEditorIsValid({ ...draft, title: " has whitespace " }), true);
  assert.equal(announcementEditorIsValid({ ...draft, title: "has\nnewline" }), false);
  assert.equal(announcementEditorIsValid({ ...draft, title: "unsafe\u0000" }), false);
  assert.equal(announcementEditorIsValid({ ...draft, scheduledForLocal: "2020-01-01T00:00" }), false);
  assert.equal(announcementEditorIsValid({ ...draft, scheduledForLocal: "2099-01-01T00:00" }), true);
  assert.throws(() => announcementEditorInput({ ...draft, markdown: "\n" }), /invalid contest announcement/);
});

test("clarification content requires a public summary only for participant-visible answers", () => {
  const common = { answer_markdown: "Answer", reason: "Reviewed" };
  assert.equal(clarificationAnswerIsValid({ ...common, audience: "requester" }), true);
  assert.equal(clarificationAnswerIsValid({ ...common, audience: "participants" }), false);
  assert.equal(clarificationAnswerIsValid({ ...common, audience: "participants", public_summary_markdown: "Public summary" }), true);
  assert.equal(clarificationQuestionIsValid(" question "), true);
  assert.equal(clarificationQuestionIsValid("\u0000"), false);
  assert.equal(contestContentReasonIsValid(" reason "), true);
});

test("date conversion and UTF-8 measurement preserve transport-safe semantics", () => {
  assert.equal(announcementScheduleLocal("not-a-date"), "");
  assert.equal(announcementScheduleLocal("2026-01-01T00:00:00.000Z").length, 16);
  assert.equal(utf8Bytes("a"), 1);
  assert.equal(utf8Bytes("é"), 2);
});
