import type { EntityID, ISODateTime } from "@/types/api";

/** Public invalidation emitted when a contest standing may have changed. */
export interface ContestStandingsChangedV1 {
  contest_id: EntityID;
  submission_id: EntityID;
  verdict_version: number;
  changed_at: ISODateTime;
}

export type ContestInvalidationResource =
  | "lifecycle"
  | "announcement"
  | "clarification"
  | "rating";

export type ContestInvalidationStatus =
  | "draft"
  | "upcoming"
  | "running"
  | "ended"
  | "cancelled"
  | "published"
  | "withdrawn"
  | "open"
  | "answered"
  | "closed"
  | "applied";

/** Content-free contest invalidation; REST remains the source of truth. */
export interface ContestInvalidationV1 {
  contest_id: EntityID;
  resource: ContestInvalidationResource;
  resource_id: EntityID;
  status: ContestInvalidationStatus;
  version: number;
  changed_at: ISODateTime;
}

/**
 * Durable outbox invalidation emitted after a terminal judge verdict. The
 * event intentionally omits mutable row fields (language and actual resource
 * measurements); REST reloads the canonical submission summary.
 */
export interface SubmissionJudgedV1 {
  submission_id: EntityID;
  user_id: EntityID;
  problem_id: EntityID;
  contest_id?: EntityID;
  generation: number;
  verdict: string;
  passed_count: number;
  total_count: number;
  aggregate_version: number;
  submitted_at: ISODateTime;
  judged_at: ISODateTime;
}

/** Minimal authenticated invalidation; grants remain REST-only. */
export interface AuthorizationCapabilitiesChangedV1 {
  revision: number;
  changed_at: ISODateTime;
}
