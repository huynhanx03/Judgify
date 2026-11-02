/**
 * Submission-related types mirroring backend submission DTOs.
 * Backend status values: pending, judging, accepted, wrong_answer,
 * time_limit_exceeded, memory_limit_exceeded, process_limit_exceeded,
 * output_limit_exceeded, runtime_error, compile_error.
 */

import type {
  CorrelationID,
  Cursor,
  EntityID,
  ISODateTime,
} from "@/types/api";

/** Verdict/status of a submission — matches backend enum values. */
export type SubmissionStatus =
  | "pending"
  | "judging"
  | "accepted"
  | "wrong_answer"
  | "time_limit_exceeded"
  | "memory_limit_exceeded"
  | "process_limit_exceeded"
  | "output_limit_exceeded"
  | "runtime_error"
  | "compile_error"
  | "internal_error";

declare const runtimeKeyBrand: unique symbol;

/** Display/operational language projected by the server, never an allowlist. */
export type Language = string;

/** Opaque authority selected only from the release-activated runtime catalog. */
export type RuntimeKey = string & {
  readonly [runtimeKeyBrand]: "RuntimeKey";
};

/** Public, command-free description of one release-activated runtime. */
export interface JudgeRuntime {
  runtime_key: RuntimeKey;
  language: Language;
  display_name: string;
  source_filename: string;
  file_extensions: readonly string[];
  compiled: boolean;
}

export interface JudgeRuntimeCatalog {
  version: number;
  maximum_source_code_bytes: number;
  runtimes: readonly JudgeRuntime[];
}

/** Bounded projection returned by submission list endpoints. */
export interface SubmissionSummary {
  id: EntityID;
  problem_id: EntityID;
  user_id: EntityID;
  contest_id?: EntityID;
  language: Language;
  status: SubmissionStatus;
  passed_count: number;
  total_count: number;
  time_ms?: number;
  memory_kb?: number;
  created_at: ISODateTime;
}

/** Stable created_at/id-desc cursor page for an owner's problem history. */
export interface SubmissionCursorPage {
  items: SubmissionSummary[];
  next_cursor: Cursor | null;
  has_more: boolean;
}

export interface SubmissionCursorQuery {
  limit?: number;
  cursor?: string;
  contest_id?: string;
}

/** Owner-only detail projection returned by GET /submissions/:id. */
export interface Submission extends SubmissionSummary {
  source_code: string;
  error_message?: string;
}

/** Safe administrative monitoring projection; source and diagnostics excluded. */
export interface AdminSubmissionSummary extends SubmissionSummary {
  latest_generation_started_at?: ISODateTime;
  judged_at?: ISODateTime;
  verdict_version: number;
  updated_at: ISODateTime;
}

/** Sensitive administrative projection protected by submission:inspect. */
export interface AdminSubmissionDetail extends AdminSubmissionSummary {
  source_code: string;
  error_message?: string;
  aggregate_version: number;
  latest_generation: number;
  latest_generation_phase: SubmissionGenerationPhase;
  latest_generation_outcome?: SubmissionGenerationOutcome;
}

export interface AdminRejudgeSubmissionRequest {
  expected_version: number;
  reason: string;
}

export interface AdminCancelSubmissionRequest extends AdminRejudgeSubmissionRequest {
  generation: number;
}

/** Bounded acknowledgement for an administrative judge operation. */
export interface AdminJudgeOperationReceipt {
  submission_id: EntityID;
  generation: number;
  aggregate_version: number;
  phase: SubmissionGenerationPhase;
  outcome?: SubmissionGenerationOutcome;
  job_id?: EntityID;
  idempotent_replay: boolean;
}

export interface AdminSubmissionFindRequest {
  pagination: {
    page: number;
    page_size: number;
  };
  status?: SubmissionStatus;
  language?: Language;
  problem_id?: EntityID;
  user_id?: EntityID;
  contest_id?: EntityID;
}

interface CreateSubmissionBase {
  problem_id: EntityID;
  runtime_key: RuntimeKey;
  source_code: string;
}

/** Contest intake is valid only with the immutable contest-problem membership. */
export type CreateSubmissionRequest = CreateSubmissionBase &
  (
    | {
        contest_id?: never;
        contest_problem_id?: never;
      }
    | {
        contest_id: EntityID;
        contest_problem_id: EntityID;
      }
  );

export type SubmissionGenerationPhase =
  | "queued"
  | "claimed"
  | "compiling"
  | "running"
  | "finalizing"
  | "terminal"
  | "cancelled";

export type SubmissionGenerationOutcome =
  | "accepted"
  | "wrong_answer"
  | "compile_error"
  | "runtime_error"
  | "time_limit_exceeded"
  | "memory_limit_exceeded"
  | "process_limit_exceeded"
  | "output_limit_exceeded"
  | "system_error"
  | "cancelled";

export type ActiveSubmissionVerdict = Exclude<
  SubmissionGenerationOutcome,
  "system_error" | "cancelled"
>;

/** Durable resource snapshot returned by the asynchronous 202 intake. */
export interface SubmissionAcceptedSnapshot {
  id: EntityID;
  aggregate_version: number;
  latest_generation: number;
  latest_generation_phase: SubmissionGenerationPhase;
  latest_generation_outcome: SubmissionGenerationOutcome | null;
  active_judgement_id: EntityID | null;
  active_judgement_generation: number | null;
  active_verdict: ActiveSubmissionVerdict | null;
  submitted_at: ISODateTime;
  contest_submission_sequence?: number;
  cid: CorrelationID;
  idempotent_replay: boolean;
}

/** Test case sample shown in problem description. */
export interface TestCase {
  input: string;
  output: string;
  explanation?: string;
}

/**
 * Test case response from backend GET /problems/:id/test-cases.
 * Maps expected_output -> output for display compatibility.
 */
export interface TestCaseResponse {
  id: EntityID;
  problem_id: EntityID;
  input: string;
  expected_output: string;
  is_hidden: boolean;
  order_index: number;
}

/** Bounded public sample; hidden state and internal IDs never cross the API boundary. */
export interface SampleTestCaseResponse {
  input: string;
  expected_output: string;
  order_index: number;
}
