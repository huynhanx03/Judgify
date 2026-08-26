import {
  arraySchema,
  booleanSchema,
  enumSchema,
  integerSchema,
  nullableSchema,
  optionalSchema,
  stringSchema,
  strictObjectSchema,
  type Schema,
} from "@/lib/api/schema";
import {
  compareISODateTime,
  correlationIDSchema,
  cursorSchema,
  entityIDSchema,
  isoDateTimeSchema,
  paginatedSchema,
} from "@/lib/api/contracts";
import {
  JUDGE_CONTENT_LIMITS,
  SUBMISSION_STATUSES,
} from "@/constants/submission";
import { inspectSourceCode } from "@/lib/submissions/content-limits";
import type {
  ActiveSubmissionVerdict,
  AdminSubmissionDetail,
  AdminJudgeOperationReceipt,
  AdminSubmissionSummary,
  Language,
  Submission,
  SubmissionAcceptedSnapshot,
  SubmissionCursorPage,
  SubmissionGenerationOutcome,
  SubmissionGenerationPhase,
  SubmissionStatus,
  SubmissionSummary,
} from "@/types/submission";
import type { Paginated } from "@/types/api";

const MAXIMUM_SUBMISSION_LIST_ITEMS = 1_000;
const MAXIMUM_LANGUAGE_CHARACTERS = 64;
const MAXIMUM_ERROR_MESSAGE_CHARACTERS = 16 * 1024;

export const submissionStatusSchema = enumSchema(
  SUBMISSION_STATUSES,
) as Schema<SubmissionStatus>;

/**
 * Language is a server-projected display identifier. The activated runtime
 * catalog remains the command allowlist; read models only enforce a bounded
 * value so adding a runtime never requires a frontend deployment.
 */
export const submissionLanguageSchema = stringSchema({
  minimumLength: 1,
  maximumLength: MAXIMUM_LANGUAGE_CHARACTERS,
  pattern: /^[a-z][a-z0-9._+-]{0,63}$/,
  label: "submission language",
}) as Schema<Language>;

export const submissionResourceUsageSchema = integerSchema({
  minimum: 0,
  label: "submission resource usage",
});
export const submissionTestCountSchema = integerSchema({
  minimum: 0,
  label: "submission test count",
});
const boundedSourceCodeStringSchema = stringSchema({
  minimumLength: 1,
  maximumLength: JUDGE_CONTENT_LIMITS.MAXIMUM_SOURCE_CODE_BYTES,
  label: "submission source code",
});
const sourceCodeSchema: Schema<string> = {
  parse(value: unknown, path = "$"): string {
    const source = boundedSourceCodeStringSchema.parse(value, path);
    if (!inspectSourceCode(source).valid) {
      throw new TypeError(`${path}: invalid bounded UTF-8 source code`);
    }
    return source;
  },
};
const errorMessageSchema = stringSchema({
  maximumLength: MAXIMUM_ERROR_MESSAGE_CHARACTERS,
  label: "submission diagnostic",
});

const submissionSummaryShape = {
  id: entityIDSchema,
  problem_id: entityIDSchema,
  user_id: entityIDSchema,
  contest_id: optionalSchema(entityIDSchema),
  language: submissionLanguageSchema,
  status: submissionStatusSchema,
  passed_count: submissionTestCountSchema,
  total_count: submissionTestCountSchema,
  time_ms: optionalSchema(submissionResourceUsageSchema),
  memory_kb: optionalSchema(submissionResourceUsageSchema),
  created_at: isoDateTimeSchema,
} as const;

const adminSubmissionSummaryShape = {
  ...submissionSummaryShape,
  latest_generation_started_at: optionalSchema(isoDateTimeSchema),
  judged_at: optionalSchema(isoDateTimeSchema),
  verdict_version: integerSchema({
    minimum: 0,
    label: "submission verdict version",
  }),
  updated_at: isoDateTimeSchema,
} as const;

export function assertSubmissionCountInvariant(
  passedCount: number,
  totalCount: number,
  path: string,
): void {
  if (passedCount > totalCount) {
    throw new TypeError(
      `${path}: submission passed count exceeds total count`,
    );
  }
}

export function assertAcceptedSubmissionInvariant(
  status: SubmissionStatus,
  passedCount: number,
  totalCount: number,
  path: string,
): void {
  if (
    status === "accepted" &&
    (totalCount === 0 || passedCount !== totalCount)
  ) {
    throw new TypeError(
      `${path}: accepted submission must pass every test`,
    );
  }
}

const rawSubmissionSummarySchema = strictObjectSchema(
  submissionSummaryShape,
);

export const submissionSummarySchema: Schema<SubmissionSummary> = {
  parse(value: unknown, path = "$"): SubmissionSummary {
    const submission = rawSubmissionSummarySchema.parse(value, path);
    assertSubmissionCountInvariant(
      submission.passed_count,
      submission.total_count,
      path,
    );
    assertAcceptedSubmissionInvariant(
      submission.status,
      submission.passed_count,
      submission.total_count,
      path,
    );
    return submission;
  },
};

export const submissionSummaryListSchema: Schema<SubmissionSummary[]> =
  arraySchema(submissionSummarySchema, {
    maximumLength: MAXIMUM_SUBMISSION_LIST_ITEMS,
  });

const rawSubmissionCursorPageSchema = strictObjectSchema({
  items: arraySchema(submissionSummarySchema, { maximumLength: 50 }),
  next_cursor: nullableSchema(cursorSchema),
  has_more: booleanSchema,
});

export const submissionCursorPageSchema: Schema<SubmissionCursorPage> = {
  parse(value: unknown, path = "$"): SubmissionCursorPage {
    const page = rawSubmissionCursorPageSchema.parse(value, path);
    if (
      page.has_more !== (page.next_cursor !== null) ||
      (page.has_more && page.items.length === 0)
    ) {
      throw new TypeError(`${path}: inconsistent submission cursor page`);
    }
    const seen = new Set<string>();
    for (let index = 0; index < page.items.length; index += 1) {
      const current = page.items[index];
      if (seen.has(current.id)) {
        throw new TypeError(`${path}: duplicate submission in cursor page`);
      }
      seen.add(current.id);
      const previous = page.items[index - 1];
      if (!previous) continue;
      const temporalOrder = compareISODateTime(
        previous.created_at,
        current.created_at,
      );
      if (
        temporalOrder < 0 ||
        (temporalOrder === 0 &&
          previous.id < current.id)
      ) {
        throw new TypeError(
          `${path}: submission cursor page is not stably ordered`,
        );
      }
    }
    return page;
  },
};

const rawSubmissionSchema = strictObjectSchema({
  ...submissionSummaryShape,
  source_code: sourceCodeSchema,
  error_message: optionalSchema(errorMessageSchema),
});

export const submissionSchema: Schema<Submission> = {
  parse(value: unknown, path = "$"): Submission {
    const submission = rawSubmissionSchema.parse(value, path);
    assertSubmissionCountInvariant(
      submission.passed_count,
      submission.total_count,
      path,
    );
    assertAcceptedSubmissionInvariant(
      submission.status,
      submission.passed_count,
      submission.total_count,
      path,
    );
    return submission;
  },
};

const rawAdminSubmissionSummarySchema = strictObjectSchema(
  adminSubmissionSummaryShape,
);

export const adminSubmissionSummarySchema: Schema<AdminSubmissionSummary> = {
  parse(value: unknown, path = "$"): AdminSubmissionSummary {
    const submission = rawAdminSubmissionSummarySchema.parse(value, path);
    assertSubmissionCountInvariant(
      submission.passed_count,
      submission.total_count,
      path,
    );
    assertAcceptedSubmissionInvariant(
      submission.status,
      submission.passed_count,
      submission.total_count,
      path,
    );
    return submission;
  },
};

export const adminSubmissionPageSchema: Schema<
  Paginated<AdminSubmissionSummary>
> = paginatedSchema(adminSubmissionSummarySchema);

const detailGenerationPhaseSchema = enumSchema([
  "queued",
  "claimed",
  "compiling",
  "running",
  "finalizing",
  "terminal",
  "cancelled",
] as const) as Schema<SubmissionGenerationPhase>;

const detailGenerationOutcomeSchema = enumSchema([
  "accepted",
  "wrong_answer",
  "compile_error",
  "runtime_error",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "process_limit_exceeded",
  "output_limit_exceeded",
  "system_error",
  "cancelled",
] as const) as Schema<SubmissionGenerationOutcome>;

const rawAdminSubmissionDetailSchema = strictObjectSchema({
  ...adminSubmissionSummaryShape,
  source_code: sourceCodeSchema,
  error_message: optionalSchema(errorMessageSchema),
  aggregate_version: integerSchema({
    minimum: 1,
    label: "submission aggregate version",
  }),
  latest_generation: integerSchema({
    minimum: 1,
    label: "submission generation",
  }),
  latest_generation_phase: detailGenerationPhaseSchema,
  latest_generation_outcome: optionalSchema(detailGenerationOutcomeSchema),
});

function assertGenerationProjectionInvariant(
  submission: Pick<
    AdminSubmissionDetail,
    "latest_generation_phase" | "latest_generation_outcome"
  >,
  path: string,
): void {
  if (
    (submission.latest_generation_phase === "terminal" &&
      submission.latest_generation_outcome === undefined) ||
    (submission.latest_generation_phase === "cancelled" &&
      submission.latest_generation_outcome !== "cancelled") ||
    (submission.latest_generation_phase !== "terminal" &&
      submission.latest_generation_phase !== "cancelled" &&
      submission.latest_generation_outcome !== undefined)
  ) {
    throw new TypeError(`${path}: invalid latest generation state`);
  }
}

export const adminSubmissionDetailSchema: Schema<AdminSubmissionDetail> = {
  parse(value: unknown, path = "$"): AdminSubmissionDetail {
    const submission = rawAdminSubmissionDetailSchema.parse(value, path);
    assertSubmissionCountInvariant(
      submission.passed_count,
      submission.total_count,
      path,
    );
    assertAcceptedSubmissionInvariant(
      submission.status,
      submission.passed_count,
      submission.total_count,
      path,
    );
    assertGenerationProjectionInvariant(submission, path);
    return submission;
  },
};

const generationPhaseSchema = enumSchema([
  "queued",
  "claimed",
  "compiling",
  "running",
  "finalizing",
  "terminal",
  "cancelled",
] as const) as Schema<SubmissionGenerationPhase>;

const generationOutcomeSchema = enumSchema([
  "accepted",
  "wrong_answer",
  "compile_error",
  "runtime_error",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "process_limit_exceeded",
  "output_limit_exceeded",
  "system_error",
  "cancelled",
] as const) as Schema<SubmissionGenerationOutcome>;

const activeVerdictSchema = enumSchema([
  "accepted",
  "wrong_answer",
  "compile_error",
  "runtime_error",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "process_limit_exceeded",
  "output_limit_exceeded",
] as const) as Schema<ActiveSubmissionVerdict>;

const rawAdminJudgeOperationReceiptSchema = strictObjectSchema({
  submission_id: entityIDSchema,
  generation: integerSchema({ minimum: 1, label: "operation generation" }),
  aggregate_version: integerSchema({
    minimum: 1,
    label: "operation aggregate version",
  }),
  phase: generationPhaseSchema,
  outcome: optionalSchema(generationOutcomeSchema),
  job_id: optionalSchema(entityIDSchema),
  idempotent_replay: booleanSchema,
});

export const adminJudgeOperationReceiptSchema: Schema<AdminJudgeOperationReceipt> = {
  parse(value: unknown, path = "$"): AdminJudgeOperationReceipt {
    const receipt = rawAdminJudgeOperationReceiptSchema.parse(value, path);
    if (
      (receipt.phase === "terminal" && receipt.outcome === undefined) ||
      (receipt.phase === "cancelled" && receipt.outcome !== "cancelled") ||
      (receipt.phase !== "terminal" &&
        receipt.phase !== "cancelled" &&
        receipt.outcome !== undefined)
    ) {
      throw new TypeError(`${path}: invalid operation outcome for phase`);
    }
    return receipt;
  },
};

const rawAcceptedSnapshotSchema = strictObjectSchema({
  id: entityIDSchema,
  aggregate_version: integerSchema({
    minimum: 1,
    label: "submission aggregate version",
  }),
  latest_generation: integerSchema({
    minimum: 1,
    label: "submission generation",
  }),
  latest_generation_phase: generationPhaseSchema,
  latest_generation_outcome: nullableSchema(generationOutcomeSchema),
  active_judgement_id: nullableSchema(entityIDSchema),
  active_judgement_generation: nullableSchema(
    integerSchema({ minimum: 1, label: "active judgement generation" }),
  ),
  active_verdict: nullableSchema(activeVerdictSchema),
  submitted_at: isoDateTimeSchema,
  contest_submission_sequence: optionalSchema(
    integerSchema({ minimum: 1, label: "contest submission sequence" }),
  ),
  cid: correlationIDSchema,
  idempotent_replay: booleanSchema,
});

export const submissionAcceptedSnapshotSchema: Schema<SubmissionAcceptedSnapshot> = {
  parse(value: unknown, path = "$"): SubmissionAcceptedSnapshot {
    const snapshot = rawAcceptedSnapshotSchema.parse(value, path);
    const activeFields = [
      snapshot.active_judgement_id,
      snapshot.active_judgement_generation,
      snapshot.active_verdict,
    ];
    const activeCount = activeFields.filter((entry) => entry !== null).length;
    if (activeCount !== 0 && activeCount !== activeFields.length) {
      throw new TypeError(`${path}: active judgement projection is partial`);
    }
    if (
      snapshot.active_judgement_generation !== null &&
      snapshot.active_judgement_generation > snapshot.latest_generation
    ) {
      throw new TypeError(`${path}: active generation exceeds latest generation`);
    }
    if (
      snapshot.latest_generation_phase === "terminal" &&
      snapshot.latest_generation_outcome === null
    ) {
      throw new TypeError(`${path}: terminal generation is missing an outcome`);
    }
    if (
      snapshot.latest_generation_phase === "cancelled" &&
      snapshot.latest_generation_outcome !== "cancelled"
    ) {
      throw new TypeError(`${path}: cancelled generation has an invalid outcome`);
    }
    if (
      snapshot.latest_generation_phase !== "terminal" &&
      snapshot.latest_generation_phase !== "cancelled" &&
      snapshot.latest_generation_outcome !== null
    ) {
      throw new TypeError(`${path}: active generation cannot have an outcome`);
    }
    return snapshot;
  },
};
