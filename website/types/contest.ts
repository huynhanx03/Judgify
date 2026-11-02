/**
 * Contest API contracts. All date/time values are canonical UTC ISO strings.
 */

import type {
  CorrelationID,
  EntityID,
  ISODateTime,
} from "@/types/api";
import type { RuntimeKey } from "@/types/submission";
import type { Operation } from "@/types/operation";

export type ContestStatus =
  | "draft"
  | "upcoming"
  | "running"
  | "ended"
  | "cancelled";

export type ContestRegistrationMode = "registered" | "open";
export type ContestScoringPolicy = "icpc";
export type ContestRegistrationStatus =
  | "registered"
  | "withdrawn"
  | "banned";

export interface ContestDraftProblem {
  problem_id: EntityID;
  problem_slug: string;
  problem_title: string;
  display_order: number;
  alias: string;
  points?: number;
  visible_before_start: boolean;
}

export interface ContestProblemMembership {
  contest_problem_id: EntityID;
  problem_id: EntityID;
  problem_slug: string;
  problem_title: string;
  display_order: number;
  alias: string;
  points?: number;
  visible_before_start: boolean;
}

export interface ContestProblemDifficulty {
  id: EntityID;
  revision: number;
  name: string;
  level: number;
  description?: string;
}

export interface ContestProblemTag {
  id: EntityID;
  revision: number;
  name: string;
}

export interface ContestProblemSample {
  input: string;
  expected_output: string;
  order_index: number;
}

/** Exact participant-facing revision frozen when the contest was published. */
export interface ContestProblemDetail {
  contest_id: EntityID;
  contest_problem_id: EntityID;
  problem_id: EntityID;
  problem_slug: string;
  revision_number: number;
  title: string;
  statement_markdown: string;
  difficulty: ContestProblemDifficulty;
  tags: ContestProblemTag[];
  cpu_time_ms: number;
  wall_time_ms: number;
  memory_limit_kb: number;
  output_limit_bytes: number;
  process_limit: number;
  allowed_runtime_keys: RuntimeKey[];
  submission_count: number;
  accepted_count: number;
  acceptance_rate: number;
  display_order: number;
  alias: string;
  points?: number;
  samples: ContestProblemSample[];
  created_at: ISODateTime;
}

export interface ContestRegistration {
  id: EntityID;
  status: ContestRegistrationStatus;
  version: number;
}

export interface Contest {
  id: EntityID;
  version: number;
  title: string;
  description: string;
  start_time: ISODateTime;
  end_time: ISODateTime;
  status: ContestStatus;
  author_id: EntityID;
  registration_mode: ContestRegistrationMode;
  registration_opens_at?: ISODateTime;
  registration_closes_at?: ISODateTime;
  max_participants?: number;
  participant_count: number;
  problem_count: number;
  scoring_policy: ContestScoringPolicy;
  scoring_policy_version: number;
  standings_projection_version: number;
  freeze_standings_at?: ISODateTime;
  rated: boolean;
  rating_algorithm_version: "expected-rank-v1";
  draft_problems: ContestDraftProblem[];
  problems: ContestProblemMembership[];
  registration?: ContestRegistration;
  is_registered?: boolean;
  published_at?: ISODateTime;
  started_at?: ISODateTime;
  ended_at?: ISODateTime;
  cancelled_at?: ISODateTime;
  rating_completed_at?: ISODateTime;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

export interface ContestDraftProblemInput {
  problem_id: EntityID;
  alias: string;
  visible_before_start: boolean;
}

export interface ContestDraftInput {
  reason: string;
  title: string;
  description: string;
  start_time: ISODateTime;
  end_time: ISODateTime;
  registration_mode: ContestRegistrationMode;
  registration_opens_at?: ISODateTime;
  registration_closes_at?: ISODateTime;
  max_participants?: number;
  freeze_standings_at?: ISODateTime;
  rated: boolean;
  problems: ContestDraftProblemInput[];
}

export interface ContestUpdateInput extends ContestDraftInput {
  expected_version: number;
}

export interface ContestLifecycleReceipt {
  contest_id: EntityID;
  from: ContestStatus;
  to: ContestStatus;
  version: number;
  occurred_at: ISODateTime;
  command_id: EntityID;
  idempotent_replay: boolean;
  cid: CorrelationID;
}

export type ContestPublishReceipt = ContestLifecycleReceipt;

export interface ContestRegistrationReceipt {
  contest_id: EntityID;
  user_id: EntityID;
  registration_id: EntityID;
  status: ContestRegistrationStatus;
  registration_version: number;
  contest_version: number;
  occurred_at: ISODateTime;
  command_id: EntityID;
  idempotent_replay: boolean;
  cid: CorrelationID;
}

export interface Standing {
  rank: number;
  user_id: EntityID;
  username: string;
  solved_count: number;
  penalty: number;
  problem_results: Record<string, unknown>;
  pending_attempts: number;
  pending_by_problem: Record<string, number>;
}

export interface StandingsSnapshot {
  contest_id: EntityID;
  view: "frozen" | "official";
  projection_version: number;
  last_cursor: string;
  frozen_at?: ISODateTime;
  items: Standing[];
}

export interface SelfStandingSnapshot {
  contest_id: EntityID;
  projection_version: number;
  last_cursor: string;
  item?: Standing;
}

export interface RatingChange {
  user_id: EntityID;
  username: string;
  old_rating: number;
  new_rating: number;
  rank: number;
  delta: number;
}

export interface RatingReratingStartResponse {
  operation: Operation;
  trigger_contest_id: EntityID;
  from_sequence: number;
  through_sequence: number;
  source_revision: number;
  target_generation_id: EntityID;
  target_run_chain_id: EntityID;
  algorithm_version: "expected-rank-v1";
  idempotent_replay: boolean;
}

export interface ContestContentPage<T> { items: T[]; next_cursor?: string; }

export interface ContestAnnouncement {
  id: EntityID;
  contest_id: EntityID;
  status: "draft" | "published" | "withdrawn";
  audience: "public" | "participants" | "jury";
  title: string;
  markdown?: string;
  sanitized_html: string;
  revision: number;
  version: number;
  scheduled_for?: ISODateTime;
  published_at?: ISODateTime;
  withdrawn_at?: ISODateTime;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

export interface ContestClarification {
  id: EntityID;
  contest_id: EntityID;
  contest_problem_id?: EntityID;
  requester_id?: EntityID;
  status: "open" | "answered" | "closed" | "withdrawn";
  audience: "requester" | "participants";
  version: number;
  question_html?: string;
  question_markdown?: string;
  public_summary_html?: string;
  answer_html?: string;
  created_at: ISODateTime;
  answered_at?: ISODateTime;
  closed_at?: ISODateTime;
  withdrawn_at?: ISODateTime;
  updated_at: ISODateTime;
}

export interface ContestContentRevision {
  id: EntityID;
  content_kind:
    | "announcement_body"
    | "question"
    | "public_question_summary"
    | "answer";
  revision_number: number;
  title?: string;
  markdown: string;
  author_id: EntityID;
  created_at: ISODateTime;
}
