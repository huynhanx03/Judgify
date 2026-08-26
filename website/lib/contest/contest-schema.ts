import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  optionalSchema,
  strictObjectSchema,
  stringSchema,
  jsonValueSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  correlationIDSchema,
  entityIDSchema,
  isoDateTimeSchema,
} from "@/lib/api/contracts";
import type {
  Contest,
  ContestDraftProblem,
  ContestProblemDetail,
  ContestProblemMembership,
  ContestLifecycleReceipt,
  ContestRegistrationReceipt,
  ContestRegistrationStatus,
  Standing,
  StandingsSnapshot,
  SelfStandingSnapshot,
  ContestStatus,
  RatingChange,
  RatingReratingStartResponse,
} from "@/types/contest";
import { runtimeKeySchema } from "@/lib/submissions/runtime-catalog";
import { operationSchema } from "@/lib/operations/operation-schema";
import { CONTEST_RATING } from "@/constants/contest";

const contestStatusSchema = enumSchema([
  "draft",
  "upcoming",
  "running",
  "ended",
  "cancelled",
] as const) as Schema<ContestStatus>;

const registrationModeSchema = enumSchema([
  "registered",
  "open",
] as const);

const registrationStatusSchema = enumSchema([
  "registered",
  "withdrawn",
  "banned",
] as const) as Schema<ContestRegistrationStatus>;

const scoringPolicySchema = enumSchema(["icpc"] as const);

const draftProblemSchema = strictObjectSchema({
  problem_id: entityIDSchema,
  problem_slug: stringSchema({
    minimumLength: 1,
    maximumLength: 128,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    label: "contest problem slug",
  }),
  problem_title: stringSchema({
    minimumLength: 1,
    maximumLength: 300,
    label: "contest problem title",
  }),
  display_order: integerSchema({
    minimum: 1,
    maximum: 500,
    label: "contest draft problem display order",
  }),
  alias: stringSchema({
    minimumLength: 1,
    maximumLength: 16,
    pattern: /^[A-Z][A-Z0-9]{0,15}$/,
    label: "contest draft problem alias",
  }),
  points: optionalSchema(integerSchema({
    minimum: 1,
    label: "contest draft problem points",
  })),
  visible_before_start: booleanSchema,
});

const frozenProblemSchema = strictObjectSchema({
  contest_problem_id: entityIDSchema,
  problem_id: entityIDSchema,
  problem_slug: stringSchema({
    minimumLength: 1,
    maximumLength: 128,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    label: "contest problem slug",
  }),
  problem_title: stringSchema({
    minimumLength: 1,
    maximumLength: 300,
    label: "contest problem title",
  }),
  display_order: integerSchema({
    minimum: 1,
    maximum: 500,
    label: "contest problem display order",
  }),
  alias: stringSchema({
    minimumLength: 1,
    maximumLength: 16,
    pattern: /^[A-Z][A-Z0-9]{0,15}$/,
    label: "contest problem alias",
  }),
  points: optionalSchema(integerSchema({
    minimum: 1,
    label: "contest problem points",
  })),
  visible_before_start: booleanSchema,
});

const registrationSchema = strictObjectSchema({
  id: entityIDSchema,
  status: registrationStatusSchema,
  version: integerSchema({
    minimum: 1,
    label: "contest registration version",
  }),
});

const rawContestSchema = strictObjectSchema({
  id: entityIDSchema,
  version: integerSchema({
    minimum: 1,
    label: "contest aggregate version",
  }),
  title: stringSchema({
    minimumLength: 1,
    maximumLength: 300,
    label: "contest title",
  }),
  description: stringSchema({
    maximumLength: 100_000,
    label: "contest description",
  }),
  start_time: isoDateTimeSchema,
  end_time: isoDateTimeSchema,
  status: contestStatusSchema,
  author_id: entityIDSchema,
  registration_mode: registrationModeSchema,
  registration_opens_at: optionalSchema(isoDateTimeSchema),
  registration_closes_at: optionalSchema(isoDateTimeSchema),
  max_participants: optionalSchema(integerSchema({
    minimum: 1,
    maximum: 1_000_000,
    label: "maximum contest participants",
  })),
  participant_count: integerSchema({
    minimum: 0,
    label: "contest participant count",
  }),
  problem_count: integerSchema({
    minimum: 0,
    maximum: 500,
    label: "contest problem count",
  }),
  scoring_policy: scoringPolicySchema,
  scoring_policy_version: integerSchema({
    minimum: 1,
    label: "contest scoring policy version",
  }),
  standings_projection_version: integerSchema({
    minimum: 1,
    label: "contest standings projection version",
  }),
  freeze_standings_at: optionalSchema(isoDateTimeSchema),
  rated: booleanSchema,
  rating_algorithm_version: enumSchema([
    "expected-rank-v1",
  ] as const),
  draft_problems: optionalSchema(arraySchema(draftProblemSchema, {
    maximumLength: 500,
  })),
  problems: optionalSchema(arraySchema(frozenProblemSchema, {
    maximumLength: 500,
  })),
  registration: optionalSchema(registrationSchema),
  is_registered: optionalSchema(booleanSchema),
  published_at: optionalSchema(isoDateTimeSchema),
  started_at: optionalSchema(isoDateTimeSchema),
  ended_at: optionalSchema(isoDateTimeSchema),
  cancelled_at: optionalSchema(isoDateTimeSchema),
  rating_completed_at: optionalSchema(isoDateTimeSchema),
  created_at: isoDateTimeSchema,
  updated_at: isoDateTimeSchema,
});

function assertUniqueProblems(
  problems: Array<ContestDraftProblem | ContestProblemMembership>,
  path: string,
): void {
  const problemIDs = new Set<string>();
  const aliases = new Set<string>();
  const displayOrders = new Set<number>();
  const frozenIDs = new Set<string>();
  for (const problem of problems) {
    if (
      problemIDs.has(problem.problem_id) ||
      aliases.has(problem.alias) ||
      displayOrders.has(problem.display_order) ||
      ("contest_problem_id" in problem &&
        frozenIDs.has(problem.contest_problem_id))
    ) {
      throw new TypeError(`${path}: duplicate contest problem membership`);
    }
    problemIDs.add(problem.problem_id);
    aliases.add(problem.alias);
    displayOrders.add(problem.display_order);
    if ("contest_problem_id" in problem) {
      frozenIDs.add(problem.contest_problem_id);
    }
  }
  const ordered = [...displayOrders].sort((left, right) => left - right);
  if (ordered.some((value, index) => value !== index + 1)) {
    throw new TypeError(`${path}: display order must be contiguous`);
  }
}

export const contestSchema: Schema<Contest> = {
  parse(value: unknown, path = "$"): Contest {
    const parsed = rawContestSchema.parse(value, path);
    const start = Date.parse(parsed.start_time);
    const end = Date.parse(parsed.end_time);
    if (start >= end) {
      throw new TypeError(`${path}: contest time window is invalid`);
    }
    if (
      parsed.max_participants !== undefined &&
      parsed.participant_count > parsed.max_participants
    ) {
      throw new TypeError(`${path}: participant count exceeds capacity`);
    }
    if (parsed.registration_mode === "registered") {
      if (
        parsed.registration_opens_at === undefined ||
        parsed.registration_closes_at === undefined ||
        Date.parse(parsed.registration_opens_at) >
          Date.parse(parsed.registration_closes_at) ||
        Date.parse(parsed.registration_closes_at) > start
      ) {
        throw new TypeError(`${path}: registration window is invalid`);
      }
    } else if (
      parsed.registration_opens_at !== undefined ||
      parsed.registration_closes_at !== undefined
    ) {
      throw new TypeError(`${path}: open contest cannot have registration dates`);
    }
    if (
      parsed.freeze_standings_at !== undefined &&
      (
        Date.parse(parsed.freeze_standings_at) < start ||
        Date.parse(parsed.freeze_standings_at) >= end
      )
    ) {
      throw new TypeError(`${path}: standings freeze is outside the contest`);
    }
    if (
      parsed.rating_completed_at !== undefined &&
      (
        !parsed.rated ||
        parsed.status !== "ended" ||
        parsed.ended_at === undefined ||
        Date.parse(parsed.rating_completed_at) <
          Date.parse(parsed.ended_at)
      )
    ) {
      throw new TypeError(`${path}: rating completion is invalid`);
    }

    const draftProblems = parsed.draft_problems ?? [];
    const problems = parsed.problems ?? [];
    assertUniqueProblems(draftProblems, `${path}.draft_problems`);
    assertUniqueProblems(problems, `${path}.problems`);
    if (
      parsed.status === "draft" &&
      draftProblems.length > 0 &&
      parsed.problem_count !== draftProblems.length
    ) {
      throw new TypeError(`${path}: draft problem count disagrees`);
    }
    if (
      problems.length > 0 &&
      parsed.problem_count !== problems.length
    ) {
      throw new TypeError(`${path}: frozen problem count disagrees`);
    }
    const isRegistered =
      parsed.registration?.status === "registered";
    if (
      parsed.is_registered !== undefined &&
      parsed.is_registered !== isRegistered
    ) {
      throw new TypeError(`${path}: registration projections disagree`);
    }

    return {
      ...parsed,
      draft_problems: draftProblems,
      problems,
      ...(parsed.is_registered === undefined
        ? {}
        : { is_registered: parsed.is_registered }),
    };
  },
};

const rawStandingSchema = strictObjectSchema({
  rank: integerSchema({ minimum: 1, label: "standing rank" }),
  user_id: entityIDSchema,
  username: stringSchema({
    minimumLength: 1,
    maximumLength: 64,
    label: "standing username",
  }),
  solved_count: integerSchema({
    minimum: 0,
    label: "standing solved count",
  }),
  penalty: integerSchema({
    minimum: 0,
    label: "standing penalty",
  }),
  problem_results: optionalSchema(jsonValueSchema),
  pending_attempts: integerSchema({
    minimum: 0,
    label: "standing pending attempts",
  }),
  pending_by_problem: optionalSchema(jsonValueSchema),
});

const rawStandingsSnapshotSchema = strictObjectSchema({
  contest_id: entityIDSchema,
  view: enumSchema(["frozen", "official"] as const),
  projection_version: integerSchema({
    minimum: 1,
    label: "standings projection version",
  }),
  last_cursor: stringSchema({
    minimumLength: 1,
    maximumLength: 32,
    pattern: /^[1-9][0-9]*$/,
    label: "standings cursor",
  }),
  frozen_at: optionalSchema(isoDateTimeSchema),
  items: arraySchema(rawStandingSchema, { maximumLength: 500 }),
});

function standingRecord(
  value: unknown,
  path: string,
): Record<string, unknown> {
  if (
    value === undefined ||
    typeof value !== "object" ||
    value === null ||
    Array.isArray(value)
  ) {
    if (value === undefined) return {};
    throw new TypeError(`${path}: expected object`);
  }
  return value as Record<string, unknown>;
}

function pendingAttemptRecord(
  value: unknown,
  path: string,
): Record<string, number> {
  const record = standingRecord(value, path);
  const parsed: Record<string, number> = {};
  for (const [problemID, count] of Object.entries(record)) {
    entityIDSchema.parse(problemID, `${path}.${problemID}`);
    if (!Number.isSafeInteger(count) || (count as number) < 1) {
      throw new TypeError(`${path}.${problemID}: invalid pending count`);
    }
    parsed[problemID] = count as number;
  }
  return parsed;
}

function parseStandingRow(
  row: ReturnType<typeof rawStandingSchema.parse>,
  path: string,
): Standing {
  const problemResults = standingRecord(
    row.problem_results,
    `${path}.problem_results`,
  );
  for (const problemID of Object.keys(problemResults)) {
    entityIDSchema.parse(
      problemID,
      `${path}.problem_results.${problemID}`,
    );
  }
  const pendingByProblem = pendingAttemptRecord(
    row.pending_by_problem,
    `${path}.pending_by_problem`,
  );
  const pendingTotal = Object.values(pendingByProblem).reduce(
    (total, count) => total + count,
    0,
  );
  if (pendingTotal !== row.pending_attempts) {
    throw new TypeError(`${path}: inconsistent pending attempts`);
  }
  return {
    ...row,
    problem_results: problemResults,
    pending_by_problem: pendingByProblem,
  };
}

export const standingsSnapshotSchema: Schema<StandingsSnapshot> = {
  parse(value: unknown, path = "$"): StandingsSnapshot {
    const snapshot = rawStandingsSnapshotSchema.parse(value, path);
    if (
      snapshot.last_cursor !== String(snapshot.projection_version) ||
      (snapshot.view === "frozen") !==
        (snapshot.frozen_at !== undefined)
    ) {
      throw new TypeError(`${path}: inconsistent standings visibility`);
    }
    const seen = new Set<string>();
    let previousRank = 0;
    const items: Standing[] = snapshot.items.map((row, index) => {
      if (seen.has(row.user_id) || row.rank < previousRank) {
        throw new TypeError(`${path}.items[${index}]: invalid ordering`);
      }
      seen.add(row.user_id);
      previousRank = row.rank;
      const item = parseStandingRow(
        row,
        `${path}.items[${index}]`,
      );
      if (
        snapshot.view === "official" && item.pending_attempts !== 0
      ) {
        throw new TypeError(
          `${path}.items[${index}]: inconsistent pending attempts`,
        );
      }
      return item;
    });
    return { ...snapshot, items };
  },
};

const rawSelfStandingSnapshotSchema = strictObjectSchema({
  contest_id: entityIDSchema,
  projection_version: integerSchema({
    minimum: 1,
    label: "self standing projection version",
  }),
  last_cursor: stringSchema({
    minimumLength: 1,
    maximumLength: 32,
    pattern: /^[1-9][0-9]*$/,
    label: "self standing cursor",
  }),
  item: optionalSchema(rawStandingSchema),
});

export const selfStandingSnapshotSchema: Schema<SelfStandingSnapshot> = {
  parse(value: unknown, path = "$"): SelfStandingSnapshot {
    const snapshot = rawSelfStandingSnapshotSchema.parse(value, path);
    if (snapshot.last_cursor !== String(snapshot.projection_version)) {
      throw new TypeError(`${path}: inconsistent self standing cursor`);
    }
    return {
      contest_id: snapshot.contest_id,
      projection_version: snapshot.projection_version,
      last_cursor: snapshot.last_cursor,
      ...(snapshot.item
        ? { item: parseStandingRow(snapshot.item, `${path}.item`) }
        : {}),
    };
  },
};

const rawRatingChangeSchema = strictObjectSchema({
  user_id: entityIDSchema,
  username: stringSchema({
    minimumLength: 1,
    maximumLength: 64,
    label: "rated username",
  }),
  old_rating: integerSchema({
    minimum: 0,
    label: "previous rating",
  }),
  new_rating: integerSchema({
    minimum: 0,
    label: "new rating",
  }),
  rank: integerSchema({
    minimum: 1,
    maximum: 10_000,
    label: "rating result rank",
  }),
  delta: integerSchema({ label: "rating delta" }),
});

const rawRatingChangesSchema = arraySchema(rawRatingChangeSchema, {
  maximumLength: 10_000,
});

export const ratingChangesSchema: Schema<RatingChange[]> = {
  parse(value: unknown, path = "$"): RatingChange[] {
    const changes = rawRatingChangesSchema.parse(value, path);
    const seen = new Set<string>();
    let previousRank = 0;
    for (const [index, change] of changes.entries()) {
      if (
        seen.has(change.user_id) ||
        change.rank < previousRank ||
        change.new_rating - change.old_rating !== change.delta
      ) {
        throw new TypeError(`${path}[${index}]: invalid rating result`);
      }
      seen.add(change.user_id);
      previousRank = change.rank;
    }
    return changes;
  },
};

const rawRatingReratingStartSchema = strictObjectSchema({
  operation: operationSchema,
  trigger_contest_id: entityIDSchema,
  from_sequence: integerSchema({
    minimum: 1,
    maximum: CONTEST_RATING.MAXIMUM_RERATING_SEQUENCES,
    label: "rerating start sequence",
  }),
  through_sequence: integerSchema({
    minimum: 1,
    maximum: CONTEST_RATING.MAXIMUM_RERATING_SEQUENCES,
    label: "rerating through sequence",
  }),
  source_revision: integerSchema({
    minimum: 1,
    label: "rating source revision",
  }),
  target_generation_id: entityIDSchema,
  target_run_chain_id: entityIDSchema,
  algorithm_version: enumSchema(["expected-rank-v1"] as const),
  idempotent_replay: booleanSchema,
});

export const ratingReratingStartSchema:
  Schema<RatingReratingStartResponse> = {
    parse(value: unknown, path = "$"): RatingReratingStartResponse {
      const result = rawRatingReratingStartSchema.parse(value, path);
      if (
        result.operation.kind !==
          CONTEST_RATING.RERATING_OPERATION_KIND ||
        result.operation.total !== result.through_sequence ||
        result.from_sequence > result.through_sequence ||
        result.target_generation_id === result.target_run_chain_id
      ) {
        throw new TypeError(`${path}: invalid rating rebuild receipt`);
      }
      return result;
    },
  };

const percentageSchema: Schema<number> = {
  parse(value: unknown, path = "$"): number {
    if (
      typeof value !== "number" ||
      !Number.isFinite(value) ||
      value < 0 ||
      value > 100
    ) {
      throw new TypeError(`${path}: invalid percentage`);
    }
    return value;
  },
};

const contestProblemDifficultySchema = strictObjectSchema({
  id: entityIDSchema,
  revision: integerSchema({
    minimum: 1,
    label: "difficulty revision",
  }),
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 50,
    label: "difficulty name",
  }),
  level: integerSchema({
    minimum: 1,
    label: "difficulty level",
  }),
  description: optionalSchema(stringSchema({
    maximumLength: 255,
    label: "difficulty description",
  })),
});

const contestProblemTagSchema = strictObjectSchema({
  id: entityIDSchema,
  revision: integerSchema({
    minimum: 1,
    label: "tag revision",
  }),
  name: stringSchema({
    minimumLength: 1,
    maximumLength: 50,
    label: "tag name",
  }),
});

const contestProblemSampleSchema = strictObjectSchema({
  input: stringSchema({
    maximumLength: 1 << 20,
    label: "sample input",
  }),
  expected_output: stringSchema({
    maximumLength: 1 << 20,
    label: "sample expected output",
  }),
  order_index: integerSchema({
    minimum: 0,
    maximum: 255,
    label: "sample order",
  }),
});

const rawContestProblemDetailSchema = strictObjectSchema({
  contest_id: entityIDSchema,
  contest_problem_id: entityIDSchema,
  problem_id: entityIDSchema,
  problem_slug: stringSchema({
    minimumLength: 1,
    maximumLength: 128,
    pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/,
    label: "contest problem slug",
  }),
  revision_number: integerSchema({
    minimum: 1,
    label: "problem revision",
  }),
  title: stringSchema({
    minimumLength: 1,
    maximumLength: 300,
    label: "contest problem title",
  }),
  statement_markdown: stringSchema({
    minimumLength: 1,
    maximumLength: 1 << 20,
    label: "contest problem statement",
  }),
  difficulty: contestProblemDifficultySchema,
  tags: arraySchema(contestProblemTagSchema, {
    maximumLength: 64,
  }),
  cpu_time_ms: integerSchema({
    minimum: 10,
    maximum: 30_000,
    label: "CPU time limit",
  }),
  wall_time_ms: integerSchema({
    minimum: 10,
    maximum: 60_000,
    label: "wall time limit",
  }),
  memory_limit_kb: integerSchema({
    minimum: 16 << 10,
    maximum: 2 << 20,
    label: "memory limit",
  }),
  output_limit_bytes: integerSchema({
    minimum: 1,
    maximum: 1 << 20,
    label: "output limit",
  }),
  process_limit: integerSchema({
    minimum: 1,
    maximum: 64,
    label: "process limit",
  }),
  allowed_runtime_keys: arraySchema(runtimeKeySchema, {
    minimumLength: 1,
    maximumLength: 16,
  }),
  submission_count: integerSchema({
    minimum: 0,
    label: "submission count",
  }),
  accepted_count: integerSchema({
    minimum: 0,
    label: "accepted count",
  }),
  acceptance_rate: percentageSchema,
  display_order: integerSchema({
    minimum: 1,
    maximum: 500,
    label: "contest problem display order",
  }),
  alias: stringSchema({
    minimumLength: 1,
    maximumLength: 16,
    pattern: /^[A-Z][A-Z0-9]{0,15}$/,
    label: "contest problem alias",
  }),
  points: optionalSchema(integerSchema({
    minimum: 1,
    label: "contest problem points",
  })),
  samples: arraySchema(contestProblemSampleSchema, {
    maximumLength: 256,
  }),
  created_at: isoDateTimeSchema,
});

export const contestProblemDetailSchema: Schema<ContestProblemDetail> = {
  parse(value: unknown, path = "$"): ContestProblemDetail {
    const parsed = rawContestProblemDetailSchema.parse(value, path);
    if (
      parsed.wall_time_ms < parsed.cpu_time_ms ||
      parsed.accepted_count > parsed.submission_count
    ) {
      throw new TypeError(`${path}: invalid immutable problem counters or limits`);
    }
    const expectedRate = parsed.submission_count === 0
      ? 0
      : (parsed.accepted_count * 100) / parsed.submission_count;
    if (Math.abs(parsed.acceptance_rate - expectedRate) > 1e-9) {
      throw new TypeError(`${path}: acceptance rate disagrees with counters`);
    }
    const tagIDs = new Set(parsed.tags.map((tag) => tag.id));
    if (tagIDs.size !== parsed.tags.length) {
      throw new TypeError(`${path}: duplicate immutable tag`);
    }
    const runtimeKeys = new Set(parsed.allowed_runtime_keys);
    if (runtimeKeys.size !== parsed.allowed_runtime_keys.length) {
      throw new TypeError(`${path}: duplicate allowed runtime`);
    }
    if (
      parsed.samples.some(
        (sample, index) => sample.order_index !== index,
      )
    ) {
      throw new TypeError(`${path}: sample order must be contiguous`);
    }
    return parsed;
  },
};

const rawPublishReceiptSchema = strictObjectSchema({
  contest_id: entityIDSchema,
  from: contestStatusSchema,
  to: contestStatusSchema,
  version: integerSchema({
    minimum: 1,
    label: "contest aggregate version",
  }),
  occurred_at: isoDateTimeSchema,
  command_id: entityIDSchema,
  idempotent_replay: booleanSchema,
  cid: correlationIDSchema,
});

export const contestLifecycleReceiptSchema: Schema<ContestLifecycleReceipt> = {
  parse(value: unknown, path = "$"): ContestLifecycleReceipt {
    const receipt = rawPublishReceiptSchema.parse(value, path);
    if (receipt.from === receipt.to) {
      throw new TypeError(`${path}: contest lifecycle did not transition`);
    }
    return receipt;
  },
};

export const contestPublishReceiptSchema = contestLifecycleReceiptSchema;

export const contestRegistrationReceiptSchema =
  strictObjectSchema({
    contest_id: entityIDSchema,
    user_id: entityIDSchema,
    registration_id: entityIDSchema,
    status: registrationStatusSchema,
    registration_version: integerSchema({
      minimum: 1,
      label: "contest registration version",
    }),
    contest_version: integerSchema({
      minimum: 1,
      label: "contest aggregate version",
    }),
    occurred_at: isoDateTimeSchema,
    command_id: entityIDSchema,
    idempotent_replay: booleanSchema,
    cid: correlationIDSchema,
  }) as Schema<ContestRegistrationReceipt>;
