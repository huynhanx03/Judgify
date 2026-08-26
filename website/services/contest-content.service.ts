import { CONTEST_API } from "@/constants/api/contest";
import { api } from "@/lib/api/client";
import { commandPurpose } from "@/lib/api/command-purpose";
import { entityIDSchema } from "@/lib/api/contracts";
import { runIdempotentCommand } from "@/lib/api/idempotent-command";
import { arraySchema } from "@/lib/api/schema";
import {
  announcementSchema,
  announcementPageSchema,
  clarificationPageSchema,
  clarificationSchema,
  contentRevisionSchema,
} from "@/lib/contest/content-schema";
import { contestVersionETag } from "@/lib/contest/publish";
import type {
  ContestAnnouncement,
  ContestClarification,
  ContestContentPage,
  ContestContentRevision,
} from "@/types/contest";

export interface ContestContentListQuery {
  cursor?: string;
}

export interface AdminClarificationListQuery extends ContestContentListQuery {
  status?: ContestClarification["status"];
}

export interface AdminAnnouncementListQuery extends ContestContentListQuery {
  status?: ContestAnnouncement["status"];
}

export interface AnnouncementDraftInput {
  audience: ContestAnnouncement["audience"];
  title: string;
  markdown: string;
  scheduled_for?: string;
  reason: string;
}

export interface AnswerClarificationInput {
  audience: "requester" | "participants";
  answer_markdown: string;
  public_summary_markdown?: string;
  reason: string;
}

function contentPath(
  path: string,
  query: ContestContentListQuery = {},
): string {
  const search = new URLSearchParams();
  if (query.cursor) search.set("cursor", query.cursor);
  const encoded = search.toString();
  return encoded ? `${path}?${encoded}` : path;
}

function adminContentPath(
  path: string,
  query: { cursor?: string; status?: string } = {},
): string {
  const search = new URLSearchParams();
  if (query.cursor) search.set("cursor", query.cursor);
  if (query.status) search.set("status", query.status);
  const encoded = search.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export const contestContentService = {
  announcements(
    id: string,
    participant: boolean,
    query: ContestContentListQuery = {},
    signal?: AbortSignal,
  ): Promise<ContestContentPage<ContestAnnouncement>> {
    const contestID = entityIDSchema.parse(id);
    const path = participant
      ? CONTEST_API.PARTICIPANT_ANNOUNCEMENTS(contestID)
      : CONTEST_API.ANNOUNCEMENTS(contestID);
    return api(contentPath(path, query), {
      signal,
      schema: announcementPageSchema,
    });
  },

  adminAnnouncements(
    id: string,
    query: AdminAnnouncementListQuery = {},
    signal?: AbortSignal,
  ): Promise<ContestContentPage<ContestAnnouncement>> {
    const contestID = entityIDSchema.parse(id);
    return api(
      adminContentPath(
        CONTEST_API.ADMIN_ANNOUNCEMENTS(contestID),
        query,
      ),
      { signal, schema: announcementPageSchema },
    );
  },

  adminAnnouncement(
    id: string,
    announcementID: string,
    signal?: AbortSignal,
  ): Promise<ContestAnnouncement> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(announcementID);
    return api(CONTEST_API.ADMIN_ANNOUNCEMENT(contestID, contentID), {
      signal,
      schema: announcementSchema,
    });
  },

  announcementRevisions(
    id: string,
    announcementID: string,
    signal?: AbortSignal,
  ): Promise<ContestContentRevision[]> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(announcementID);
    return api(
      CONTEST_API.ADMIN_ANNOUNCEMENT_REVISIONS(contestID, contentID),
      {
        signal,
        schema: arraySchema(contentRevisionSchema, { maximumLength: 500 }),
      },
    );
  },

  async createAnnouncement(
    id: string,
    input: AnnouncementDraftInput,
  ): Promise<ContestAnnouncement> {
    const contestID = entityIDSchema.parse(id);
    const purpose = await commandPurpose("contest-announcement-create", [
      contestID,
      input.audience,
      input.title,
      input.markdown,
      input.scheduled_for ?? null,
      input.reason,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(CONTEST_API.ANNOUNCEMENTS(contestID), {
        method: "POST",
        body: input,
        idempotencyKey,
        schema: announcementSchema,
      }),
    );
  },

  async editAnnouncement(
    id: string,
    announcementID: string,
    version: number,
    input: AnnouncementDraftInput,
  ): Promise<ContestAnnouncement> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(announcementID);
    const purpose = await commandPurpose("contest-announcement-edit", [
      contestID,
      contentID,
      version,
      input.audience,
      input.title,
      input.markdown,
      input.scheduled_for ?? null,
      input.reason,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(CONTEST_API.ANNOUNCEMENT(contestID, contentID), {
        method: "PUT",
        body: input,
        idempotencyKey,
        ifMatch: contestVersionETag(version),
        schema: announcementSchema,
      }),
    );
  },

  async publishAnnouncement(
    id: string,
    announcementID: string,
    version: number,
    reason: string,
  ): Promise<ContestAnnouncement> {
    return mutateAnnouncement(
      "contest-announcement-publish",
      id,
      announcementID,
      version,
      "publish",
      reason,
    );
  },

  async withdrawAnnouncement(
    id: string,
    announcementID: string,
    version: number,
    reason: string,
  ): Promise<ContestAnnouncement> {
    return mutateAnnouncement(
      "contest-announcement-withdraw",
      id,
      announcementID,
      version,
      "withdraw",
      reason,
    );
  },

  myClarifications(
    id: string,
    query: ContestContentListQuery = {},
    signal?: AbortSignal,
  ): Promise<ContestContentPage<ContestClarification>> {
    const contestID = entityIDSchema.parse(id);
    return api(contentPath(CONTEST_API.MY_CLARIFICATIONS(contestID), query), {
      signal,
      schema: clarificationPageSchema,
    });
  },

  participantClarifications(
    id: string,
    query: ContestContentListQuery = {},
    signal?: AbortSignal,
  ): Promise<ContestContentPage<ContestClarification>> {
    const contestID = entityIDSchema.parse(id);
    return api(contentPath(CONTEST_API.CLARIFICATIONS(contestID), query), {
      signal,
      schema: clarificationPageSchema,
    });
  },

  async submitClarification(
    id: string,
    questionMarkdown: string,
    contestProblemID?: string,
  ): Promise<ContestClarification> {
    const contestID = entityIDSchema.parse(id);
    const problemID = contestProblemID
      ? entityIDSchema.parse(contestProblemID)
      : undefined;
    const body = {
      question_markdown: questionMarkdown,
      ...(problemID ? { contest_problem_id: problemID } : {}),
    };
    const purpose = await commandPurpose("contest-question-submit", [
      contestID,
      problemID ?? null,
      questionMarkdown,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(CONTEST_API.CLARIFICATIONS(contestID), {
        method: "POST",
        body,
        idempotencyKey,
        schema: clarificationSchema,
      }),
    );
  },

  async withdrawClarification(
    id: string,
    clarificationID: string,
    version: number,
    reason: string,
  ): Promise<ContestClarification> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(clarificationID);
    const purpose = await commandPurpose("contest-question-withdraw", [
      contestID,
      contentID,
      version,
      reason,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(
        CONTEST_API.WITHDRAW_CLARIFICATION(contestID, contentID),
        {
          method: "POST",
          body: { reason },
          idempotencyKey,
          ifMatch: contestVersionETag(version),
          schema: clarificationSchema,
        },
      ),
    );
  },

  adminClarifications(
    id: string,
    query: AdminClarificationListQuery = {},
    signal?: AbortSignal,
  ): Promise<ContestContentPage<ContestClarification>> {
    const contestID = entityIDSchema.parse(id);
    return api(
      adminContentPath(
        CONTEST_API.ADMIN_CLARIFICATIONS(contestID),
        query,
      ),
      { signal, schema: clarificationPageSchema },
    );
  },

  adminClarification(
    id: string,
    clarificationID: string,
    signal?: AbortSignal,
  ): Promise<ContestClarification> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(clarificationID);
    return api(CONTEST_API.ADMIN_CLARIFICATION(contestID, contentID), {
      signal,
      schema: clarificationSchema,
    });
  },

  async answerClarification(
    id: string,
    clarificationID: string,
    version: number,
    input: AnswerClarificationInput,
  ): Promise<ContestClarification> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(clarificationID);
    const purpose = await commandPurpose("contest-question-answer", [
      contestID,
      contentID,
      version,
      input.audience,
      input.answer_markdown,
      input.public_summary_markdown ?? null,
      input.reason,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(CONTEST_API.ADMIN_CLARIFICATION_ANSWER(contestID, contentID), {
        method: "POST",
        body: input,
        idempotencyKey,
        ifMatch: contestVersionETag(version),
        schema: clarificationSchema,
      }),
    );
  },

  async closeClarification(
    id: string,
    clarificationID: string,
    version: number,
    reason: string,
  ): Promise<ContestClarification> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(clarificationID);
    const purpose = await commandPurpose("contest-question-close", [
      contestID,
      contentID,
      version,
      reason,
    ]);
    return runIdempotentCommand(purpose, (idempotencyKey) =>
      api(CONTEST_API.ADMIN_CLARIFICATION_CLOSE(contestID, contentID), {
        method: "POST",
        body: { reason },
        idempotencyKey,
        ifMatch: contestVersionETag(version),
        schema: clarificationSchema,
      }),
    );
  },

  revisions(
    id: string,
    clarificationID: string,
    signal?: AbortSignal,
  ): Promise<ContestContentRevision[]> {
    const contestID = entityIDSchema.parse(id);
    const contentID = entityIDSchema.parse(clarificationID);
    return api(
      CONTEST_API.ADMIN_CLARIFICATION_REVISIONS(contestID, contentID),
      {
        signal,
        schema: arraySchema(contentRevisionSchema, { maximumLength: 500 }),
      },
    );
  },
};

async function mutateAnnouncement(
  namespace: "contest-announcement-publish" | "contest-announcement-withdraw",
  id: string,
  announcementID: string,
  version: number,
  action: "publish" | "withdraw",
  reason: string,
): Promise<ContestAnnouncement> {
  const contestID = entityIDSchema.parse(id);
  const contentID = entityIDSchema.parse(announcementID);
  const purpose = await commandPurpose(namespace, [
    contestID,
    contentID,
    version,
    reason,
  ]);
  const path = action === "publish"
    ? CONTEST_API.ANNOUNCEMENT_PUBLISH(contestID, contentID)
    : CONTEST_API.ANNOUNCEMENT_WITHDRAW(contestID, contentID);
  return runIdempotentCommand(purpose, (idempotencyKey) =>
    api(path, {
      method: "POST",
      body: { reason },
      idempotencyKey,
      ifMatch: contestVersionETag(version),
      schema: announcementSchema,
    }),
  );
}
