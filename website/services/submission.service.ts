import { api } from "@/lib/api/client";
import { SUBMISSION_API } from "@/constants/api/submission";
import { JUDGE_API } from "@/constants/api/judge";
import {
  judgeRuntimeCatalogSchema,
  runtimeKeySchema,
} from "@/lib/submissions/runtime-catalog";
import {
  adminSubmissionDetailSchema,
  adminJudgeOperationReceiptSchema,
  adminSubmissionPageSchema,
  submissionAcceptedSnapshotSchema,
  submissionCursorPageSchema,
  submissionSchema,
  submissionSummaryListSchema,
} from "@/lib/submissions/submission-schema";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { ApiError } from "@/lib/api/error";
import type {
  AdminSubmissionDetail,
  AdminCancelSubmissionRequest,
  AdminSubmissionFindRequest,
  AdminSubmissionSummary,
  AdminJudgeOperationReceipt,
  AdminRejudgeSubmissionRequest,
  CreateSubmissionRequest,
  JudgeRuntimeCatalog,
  Submission,
  SubmissionAcceptedSnapshot,
  SubmissionCursorPage,
  SubmissionCursorQuery,
  SubmissionSummary,
} from "@/types/submission";
import type { Paginated } from "@/types/api";
import {
  cursorSchema,
  entityIDSchema,
} from "@/lib/api/contracts";
import {
  SUBMISSION_HISTORY_MAXIMUM_PAGE_SIZE,
  SUBMISSION_HISTORY_PAGE_SIZE,
  ADMIN_JUDGE_OPERATION_REASON_LIMITS,
} from "@/constants/submission";
import { submissionCommandPurpose } from "@/lib/submissions/idempotency";
import { inspectSourceCode } from "@/lib/submissions/content-limits";

function submissionHistoryEndpoint(
  problemId: string,
  query: SubmissionCursorQuery = {},
): string {
  const limit = query.limit ?? SUBMISSION_HISTORY_PAGE_SIZE;
  if (
    !Number.isInteger(limit) ||
    limit < 1 ||
    limit > SUBMISSION_HISTORY_MAXIMUM_PAGE_SIZE
  ) {
    throw new RangeError(
      "submission history page size is outside the supported boundary",
    );
  }
  const params = new URLSearchParams({ limit: String(limit) });
  if (query.cursor !== undefined) {
    params.set("cursor", cursorSchema.parse(query.cursor.trim()));
  }
  if (query.contest_id !== undefined) {
    params.set(
      "contest_id",
      entityIDSchema.parse(query.contest_id.trim()),
    );
  }
  return SUBMISSION_API.MY_BY_PROBLEM(
    entityIDSchema.parse(problemId),
    params.toString(),
  );
}

function normalizeSubmissionCommand(
  body: CreateSubmissionRequest,
): CreateSubmissionRequest {
  const problemID = entityIDSchema.parse(body.problem_id);
  const runtimeKey = runtimeKeySchema.parse(body.runtime_key);
  if (
    typeof body.source_code !== "string" ||
    !inspectSourceCode(body.source_code).valid
  ) {
    throw new TypeError("submission source code is invalid");
  }
  const hasContest = body.contest_id !== undefined;
  const hasContestProblem = body.contest_problem_id !== undefined;
  if (hasContest !== hasContestProblem) {
    throw new TypeError(
      "contest_id and contest_problem_id must be supplied together",
    );
  }
  if (!hasContest || !hasContestProblem) {
    return {
      problem_id: problemID,
      runtime_key: runtimeKey,
      source_code: body.source_code,
    };
  }
  return {
    problem_id: problemID,
    runtime_key: runtimeKey,
    source_code: body.source_code,
    contest_id: entityIDSchema.parse(body.contest_id),
    contest_problem_id: entityIDSchema.parse(body.contest_problem_id),
  };
}

function normalizeJudgeOperationReason(reason: string): string {
  const normalized = reason.trim();
  if (
    normalized.length < ADMIN_JUDGE_OPERATION_REASON_LIMITS.MINIMUM_CHARACTERS ||
    normalized.length > ADMIN_JUDGE_OPERATION_REASON_LIMITS.MAXIMUM_CHARACTERS ||
    /[\p{C}]/u.test(normalized)
  ) {
    throw new TypeError("judge operation reason is invalid");
  }
  return normalized;
}

function positiveVersion(value: number, label: string): number {
  if (!Number.isSafeInteger(value) || value < 1) {
    throw new TypeError(`${label} is invalid`);
  }
  return value;
}

export const submissionService = {
  async getRuntimeCatalog(
    signal?: AbortSignal,
  ): Promise<JudgeRuntimeCatalog> {
    return api<JudgeRuntimeCatalog, never>(JUDGE_API.RUNTIMES, {
      method: "GET",
      auth: "none",
      signal,
      schema: judgeRuntimeCatalogSchema,
    });
  },
  async submit(
    body: CreateSubmissionRequest,
  ): Promise<SubmissionAcceptedSnapshot> {
    const command = normalizeSubmissionCommand(body);
    const purpose = await submissionCommandPurpose(command);
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const accepted = await api<
        SubmissionAcceptedSnapshot,
        CreateSubmissionRequest
      >(SUBMISSION_API.CREATE, {
        method: "POST",
        body: command,
        idempotencyKey: attempt.attempt_id,
        expectedStatus: 202,
        schema: submissionAcceptedSnapshotSchema,
      });
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return accepted;
    } catch (error) {
      if (
        error instanceof ApiError &&
        !error.retryable &&
        error.status < 500
      ) {
        commandAttemptStore.resolve(purpose, attempt.attempt_id);
      }
      throw error;
    }
  },
  async getById(
    id: string,
    signal?: AbortSignal,
  ): Promise<Submission> {
    return api<Submission, never>(
      SUBMISSION_API.GET(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: submissionSchema,
      },
    );
  },
  async getMyByProblem(
    problemId: string,
    query: SubmissionCursorQuery = {},
    signal?: AbortSignal,
  ): Promise<SubmissionCursorPage> {
    return api<SubmissionCursorPage, never>(
      submissionHistoryEndpoint(problemId, query),
      {
        method: "GET",
        signal,
        schema: submissionCursorPageSchema,
      },
    );
  },
  async getByProblem(problemId: string): Promise<SubmissionSummary[]> {
    return api<SubmissionSummary[], never>(
      SUBMISSION_API.BY_PROBLEM(entityIDSchema.parse(problemId)),
      {
        method: "GET",
        schema: submissionSummaryListSchema,
      },
    );
  },
  async findAdmin(
    query: AdminSubmissionFindRequest,
    signal?: AbortSignal,
  ): Promise<Paginated<AdminSubmissionSummary>> {
    return api<
      Paginated<AdminSubmissionSummary>,
      AdminSubmissionFindRequest
    >(SUBMISSION_API.ADMIN_FIND, {
      method: "POST",
      body: query,
      signal,
      schema: adminSubmissionPageSchema,
    });
  },
  async getAdminById(
    id: string,
    signal?: AbortSignal,
  ): Promise<AdminSubmissionDetail> {
    return api<AdminSubmissionDetail, never>(
      SUBMISSION_API.ADMIN_GET(entityIDSchema.parse(id)),
      {
        method: "GET",
        signal,
        schema: adminSubmissionDetailSchema,
      },
    );
  },
  async rejudgeAdmin(
    id: string,
    request: AdminRejudgeSubmissionRequest,
  ): Promise<AdminJudgeOperationReceipt> {
    const submissionID = entityIDSchema.parse(id);
    const expectedVersion = positiveVersion(
      request.expected_version,
      "expected submission version",
    );
    const purpose = `submission-rejudge-${submissionID}-${expectedVersion}`;
    const attempt = commandAttemptStore.getOrCreate(purpose);
    const body: AdminRejudgeSubmissionRequest = {
      expected_version: expectedVersion,
      reason: normalizeJudgeOperationReason(request.reason),
    };
    try {
      const receipt = await api<
        AdminJudgeOperationReceipt,
        AdminRejudgeSubmissionRequest
      >(SUBMISSION_API.ADMIN_REJUDGE(submissionID), {
        method: "POST",
        body,
        idempotencyKey: attempt.attempt_id,
        schema: adminJudgeOperationReceiptSchema,
      });
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return receipt;
    } catch (error) {
      if (
        error instanceof ApiError &&
        !error.retryable &&
        error.status < 500
      ) {
        commandAttemptStore.resolve(purpose, attempt.attempt_id);
      }
      throw error;
    }
  },
  async cancelAdmin(
    id: string,
    request: AdminCancelSubmissionRequest,
  ): Promise<AdminJudgeOperationReceipt> {
    const submissionID = entityIDSchema.parse(id);
    const body: AdminCancelSubmissionRequest = {
      generation: positiveVersion(request.generation, "submission generation"),
      expected_version: positiveVersion(
        request.expected_version,
        "expected submission version",
      ),
      reason: normalizeJudgeOperationReason(request.reason),
    };
    return api<AdminJudgeOperationReceipt, AdminCancelSubmissionRequest>(
      SUBMISSION_API.ADMIN_CANCEL(submissionID),
      {
        method: "POST",
        body,
        schema: adminJudgeOperationReceiptSchema,
      },
    );
  },
};
