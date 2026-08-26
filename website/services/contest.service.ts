import { api } from "@/lib/api/client";
import { voidSchema } from "@/lib/api/schema";
import { CONTEST_API } from "@/constants/api/contest";
import {
  entityIDSchema,
  paginatedSchema,
} from "@/lib/api/contracts";
import type { Paginated, QueryOptions } from "@/types/api";
import type {
  Contest,
  ContestDraftInput,
  ContestLifecycleReceipt,
  ContestProblemDetail,
  ContestRegistrationReceipt,
  ContestStatus,
  ContestUpdateInput,
  StandingsSnapshot,
  RatingChange,
  RatingReratingStartResponse,
  SelfStandingSnapshot,
} from "@/types/contest";
import {
  contestLifecycleReceiptSchema,
  contestProblemDetailSchema,
  contestRegistrationReceiptSchema,
  contestSchema,
  ratingChangesSchema,
  ratingReratingStartSchema,
  standingsSnapshotSchema,
  selfStandingSnapshotSchema,
} from "@/lib/contest/contest-schema";
import { commandAttemptStore } from "@/lib/api/idempotency";
import { ApiError } from "@/lib/api/error";
import {
  contestVersionETag,
  normalizeContestReason,
  normalizeRatingReratingReason,
} from "@/lib/contest/publish";

export const contestService = {
  // Read
  async find(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<Contest>> {
    return api<Paginated<Contest>, QueryOptions | undefined>(
      CONTEST_API.FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: paginatedSchema(contestSchema),
      },
    );
  },
  async getById(id: string, signal?: AbortSignal): Promise<Contest> {
    return api<Contest>(CONTEST_API.GET(entityIDSchema.parse(id)), {
      signal,
      schema: contestSchema,
    });
  },
  async getProblem(
    id: string,
    contestProblemID: string,
    signal?: AbortSignal,
  ): Promise<ContestProblemDetail> {
    const contestID = entityIDSchema.parse(id);
    const membershipID = entityIDSchema.parse(contestProblemID);
    const problem = await api<ContestProblemDetail, never>(
      CONTEST_API.PROBLEM(
        contestID,
        membershipID,
      ),
      {
        method: "GET",
        auth: "none",
        signal,
        schema: contestProblemDetailSchema,
      },
    );
    if (
      problem.contest_id !== contestID ||
      problem.contest_problem_id !== membershipID
    ) {
      throw new TypeError("contest problem response identity mismatch");
    }
    return problem;
  },

  // Administrative reads include draft and published lifecycle states.
  async findAdmin(
    query?: QueryOptions,
    signal?: AbortSignal,
  ): Promise<Paginated<Contest>> {
    return api<Paginated<Contest>, QueryOptions | undefined>(
      CONTEST_API.ADMIN_FIND,
      {
        method: "POST",
        body: query,
        signal,
        schema: paginatedSchema(contestSchema),
      },
    );
  },
  async getAdminById(
    id: string,
    signal?: AbortSignal,
  ): Promise<Contest> {
    return api<Contest>(
      CONTEST_API.ADMIN_GET(entityIDSchema.parse(id)),
      { signal, schema: contestSchema },
    );
  },

  // User actions
  async register(
    id: string,
    contestVersion: number,
    registrationVersion = 0,
  ): Promise<ContestRegistrationReceipt> {
    return mutateRegistration(
      id,
      contestVersion,
      registrationVersion,
      "register",
    );
  },
  async unregister(
    id: string,
    contestVersion: number,
    registrationVersion: number,
  ): Promise<ContestRegistrationReceipt> {
    return mutateRegistration(
      id,
      contestVersion,
      registrationVersion,
      "unregister",
    );
  },
  async getStandings(
    id: string,
    signal?: AbortSignal,
  ): Promise<StandingsSnapshot> {
    const contestID = entityIDSchema.parse(id);
    const snapshot = await api<StandingsSnapshot>(
      CONTEST_API.STANDINGS(contestID),
      { signal, schema: standingsSnapshotSchema },
    );
    if (snapshot.contest_id !== contestID) {
      throw new TypeError("standings response identity mismatch");
    }
    return snapshot;
  },
  async getAdminStandings(
    id: string,
    signal?: AbortSignal,
  ): Promise<StandingsSnapshot> {
    const contestID = entityIDSchema.parse(id);
    const snapshot = await api<StandingsSnapshot>(
      CONTEST_API.ADMIN_STANDINGS(contestID),
      { signal, schema: standingsSnapshotSchema },
    );
    if (
      snapshot.contest_id !== contestID ||
      snapshot.view !== "official"
    ) {
      throw new TypeError(
        "admin standings response visibility mismatch",
      );
    }
    return snapshot;
  },
  async getMyStanding(
    id: string,
    signal?: AbortSignal,
  ): Promise<SelfStandingSnapshot> {
    const contestID = entityIDSchema.parse(id);
    const snapshot = await api<SelfStandingSnapshot>(
      CONTEST_API.MY_STANDING(contestID),
      { signal, schema: selfStandingSnapshotSchema },
    );
    if (snapshot.contest_id !== contestID) {
      throw new TypeError("self standing response identity mismatch");
    }
    return snapshot;
  },
  async getRatingChanges(
    id: string,
    signal?: AbortSignal,
  ): Promise<RatingChange[]> {
    return api<RatingChange[]>(
      CONTEST_API.RATING_CHANGES(entityIDSchema.parse(id)),
      { signal, schema: ratingChangesSchema },
    );
  },
  async startRatingRerating(
    id: string,
    reason: string,
  ): Promise<RatingReratingStartResponse> {
    const contestID = entityIDSchema.parse(id);
    const normalizedReason = normalizeRatingReratingReason(reason);
    const purpose = `contest-rating-rerate-${contestID}`;
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      const result = await api<
        RatingReratingStartResponse,
        { reason: string }
      >(CONTEST_API.ADMIN_RERATE(contestID), {
        method: "POST",
        body: { reason: normalizedReason },
        idempotencyKey: attempt.attempt_id,
        expectedStatus: 202,
        schema: ratingReratingStartSchema,
      });
      if (result.trigger_contest_id !== contestID) {
        throw new ApiError({
          code: "invalid_response",
          status: 502,
          cid: result.operation.correlation_id,
          retryable: true,
        });
      }
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      return result;
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

  // CRUD
  async create(data: ContestDraftInput): Promise<Contest> {
    const body = {
      ...data,
      reason: normalizeContestReason(data.reason),
      problems: data.problems.map((problem) => ({
        ...problem,
        problem_id: entityIDSchema.parse(problem.problem_id),
      })),
    };
    return api<Contest, typeof body>(CONTEST_API.CREATE, {
      method: "POST",
      body,
      schema: contestSchema,
    });
  },
  async update(
    id: string,
    data: ContestUpdateInput,
  ): Promise<Contest> {
    const body = {
      ...data,
      reason: normalizeContestReason(data.reason),
      problems: data.problems.map((problem) => ({
        ...problem,
        problem_id: entityIDSchema.parse(problem.problem_id),
      })),
    };
    return api<Contest, typeof body>(
      CONTEST_API.UPDATE(entityIDSchema.parse(id)),
      {
        method: "PUT",
        body,
        schema: contestSchema,
      },
    );
  },
  async publish(
    id: string,
    expectedVersion: number,
    reason: string,
  ): Promise<ContestLifecycleReceipt> {
    return transitionContest(id, expectedVersion, reason, "upcoming");
  },
  async end(
    id: string,
    expectedVersion: number,
    reason: string,
  ): Promise<ContestLifecycleReceipt> {
    return transitionContest(id, expectedVersion, reason, "ended");
  },
  async cancel(
    id: string,
    expectedVersion: number,
    reason: string,
  ): Promise<ContestLifecycleReceipt> {
    return transitionContest(id, expectedVersion, reason, "cancelled");
  },
  async delete(id: string, reason: string): Promise<void> {
    await api<void, { reason: string }>(
      CONTEST_API.DELETE(entityIDSchema.parse(id)),
      {
        method: "DELETE",
        body: { reason: normalizeContestReason(reason) },
        schema: voidSchema,
      },
    );
  },
};

async function transitionContest(
  id: string,
  expectedVersion: number,
  reason: string,
  target: Extract<ContestStatus, "upcoming" | "ended" | "cancelled">,
): Promise<ContestLifecycleReceipt> {
  const contestID = entityIDSchema.parse(id);
  const normalizedReason = normalizeContestReason(reason);
  const endpoint =
    target === "upcoming"
      ? CONTEST_API.PUBLISH(contestID)
      : target === "ended"
        ? CONTEST_API.ADMIN_END(contestID)
        : CONTEST_API.ADMIN_CANCEL(contestID);
  const expectedSources: readonly ContestStatus[] =
    target === "upcoming"
      ? ["draft"]
      : target === "ended"
        ? ["running"]
        : ["draft", "upcoming", "running"];
  const purpose = `contest-lifecycle-${target}-${contestID}`;
  const attempt = commandAttemptStore.getOrCreate(purpose);
  try {
    const receipt = await api<
      ContestLifecycleReceipt,
      { reason: string }
    >(endpoint, {
      method: "POST",
      body: { reason: normalizedReason },
      idempotencyKey: attempt.attempt_id,
      ifMatch: contestVersionETag(expectedVersion),
      expectedStatus: 200,
      schema: contestLifecycleReceiptSchema,
    });
    if (
      receipt.contest_id !== contestID ||
      receipt.command_id !== attempt.attempt_id ||
      !expectedSources.includes(receipt.from) ||
      receipt.to !== target ||
      receipt.version !== expectedVersion + 1
    ) {
      throw new ApiError({
        code: "invalid_response",
        status: 502,
        cid: receipt.cid,
        retryable: true,
      });
    }
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
}

async function mutateRegistration(
  id: string,
  contestVersion: number,
  registrationVersion: number,
  mutation: "register" | "unregister",
): Promise<ContestRegistrationReceipt> {
  const contestID = entityIDSchema.parse(id);
  if (
    !Number.isSafeInteger(contestVersion) ||
    contestVersion < 1 ||
    !Number.isSafeInteger(registrationVersion) ||
    registrationVersion < 0
  ) {
    throw new RangeError("contest registration version is invalid");
  }
  const purpose = `contest-${mutation}-${contestID}`;
  const attempt = commandAttemptStore.getOrCreate(purpose);
  try {
    const receipt = await api<
      ContestRegistrationReceipt,
      { expected_registration_version: number }
    >(
      mutation === "register"
        ? CONTEST_API.REGISTER(contestID)
        : CONTEST_API.UNREGISTER(contestID),
      {
        method: "POST",
        body: {
          expected_registration_version: registrationVersion,
        },
        idempotencyKey: attempt.attempt_id,
        ifMatch: contestVersionETag(contestVersion),
        expectedStatus: 200,
        schema: contestRegistrationReceiptSchema,
      },
    );
    const expectedStatus =
      mutation === "register" ? "registered" : "withdrawn";
    if (
      receipt.contest_id !== contestID ||
      receipt.command_id !== attempt.attempt_id ||
      receipt.status !== expectedStatus ||
      receipt.contest_version !== contestVersion ||
      receipt.registration_version !== registrationVersion + 1
    ) {
      throw new ApiError({
        code: "invalid_response",
        status: 502,
        cid: receipt.cid,
        retryable: true,
      });
    }
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
}
