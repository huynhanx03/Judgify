/**
 * Problem-related types mirroring backend problem DTOs.
 */

import type { Tag } from "./tag";
import type { DifficultyResponse } from "./difficulty";
import type { EntityID, ISODateTime } from "@/types/api";

/** Difficulty string key — used for local styling lookups. */
export type Difficulty = "easy" | "medium" | "hard";
export type ProblemLifecycle = "draft" | "published" | "archived";

/** Problem entity returned by GET /problems/:id and POST /problems/find. */
export interface Problem {
  id: EntityID;
  slug: string;
  lifecycle: ProblemLifecycle;
  version: number;
  title: string;
  description: string;
  difficulty_id: EntityID;
  difficulty?: DifficultyResponse;
  time_limit_ms: number;
  memory_limit_kb: number;
  author_id: EntityID;
  is_published: boolean;
  submission_count: number;
  accepted_count: number;
  acceptance_rate: number;
  is_solved?: boolean;
  tags: Tag[] | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
}

/** Minimal safe statement view shared by arena and immutable contest pages. */
export interface ProblemWorkspaceStatement {
  title: string;
  description: string;
  difficulty?: {
    name: string;
    level: number;
  };
  time_limit_ms: number;
  memory_limit_kb: number;
  submission_count: number;
  acceptance_rate: number;
  tags?: ReadonlyArray<{
    id: EntityID;
    name: string;
  }> | null;
}

export type ProblemTestKind = "sample" | "hidden";
export type ProblemArtifactDiagnostic =
  | "invalid_utf8"
  | "ambiguous_encoding"
  | "source_too_large"
  | "output_too_large"
  | "nesting_too_deep"
  | "render_rejected";

export interface ProblemAuthoringTestcase {
  test_case_id: EntityID;
  kind: ProblemTestKind;
  input: string;
  expected_output: string;
}

export interface ProblemAuthoringTestcaseGroup {
  stop_on_failure: boolean;
  cases: ProblemAuthoringTestcase[];
}

export interface ProblemAuthoringDraft {
  problem_id: EntityID;
  slug: string;
  lifecycle: ProblemLifecycle;
  version: number;
  revision_id: EntityID;
  testset_revision_id: EntityID;
  title: string;
  statement_markdown: string;
  difficulty_id: EntityID;
  tag_ids: EntityID[];
  cpu_time_ms: number;
  wall_time_ms: number;
  memory_limit_kb: number;
  output_limit_bytes: number;
  process_limit: number;
  checker_key: string;
  checker_version: string;
  checker_config: Record<string, unknown>;
  allowed_runtime_keys: string[];
  artifact_status: "ready" | "rejected";
  artifact_diagnostics: ProblemArtifactDiagnostic[];
  saved_at: ISODateTime;
  groups: ProblemAuthoringTestcaseGroup[];
}

export type ProblemCheckerConfigKind = "none" | "float_tolerance";

export interface ProblemAuthoringRuntimeOption {
  key: string;
  language: string;
  display_name: string;
  profile_version: string;
}

export interface ProblemAuthoringCheckerOption {
  key: string;
  version: string;
  config_kind: ProblemCheckerConfigKind;
}

export interface ProblemAuthoringCapabilityOption {
  runtime_key: string;
  checker_key: string;
  checker_version: string;
}

export interface ProblemAuthoringNumericPolicy {
  default: number;
  minimum: number;
  maximum: number;
}

export interface ProblemAuthoringPolicy {
  slug_maximum_bytes: number;
  title_maximum_bytes: number;
  statement_maximum_bytes: number;
  reason_maximum_bytes: number;
  test_asset_maximum_bytes: number;
  maximum_testcases: number;
  maximum_groups: number;
  maximum_tags: number;
  maximum_runtimes: number;
  cpu_time_ms: ProblemAuthoringNumericPolicy;
  wall_time_ms: ProblemAuthoringNumericPolicy;
  memory_limit_kb: ProblemAuthoringNumericPolicy;
  output_limit_bytes: ProblemAuthoringNumericPolicy;
  process_limit: ProblemAuthoringNumericPolicy;
}

export interface ProblemAuthoringCatalog {
  release_id: string;
  catalog_version: string;
  runtimes: ProblemAuthoringRuntimeOption[];
  checkers: ProblemAuthoringCheckerOption[];
  capabilities: ProblemAuthoringCapabilityOption[];
  policy: ProblemAuthoringPolicy;
}

export interface SaveProblemDraftInput {
  expected_version: number;
  catalog_version: string;
  title: string;
  statement_markdown: string;
  difficulty_id: EntityID;
  tag_ids: EntityID[];
  cpu_time_ms: number;
  wall_time_ms: number;
  memory_limit_kb: number;
  output_limit_bytes: number;
  process_limit: number;
  checker_key: string;
  checker_version: string;
  checker_config: Record<string, unknown>;
  allowed_runtime_keys: string[];
  groups: Array<{
    stop_on_failure: boolean;
    cases: Array<{
      test_case_id?: EntityID;
      kind: ProblemTestKind;
      input: string;
      expected_output: string;
    }>;
  }>;
  reason: string;
}

export interface ProblemDraftReceipt {
  problem_id: EntityID;
  revision_id: EntityID;
  testset_revision_id: EntityID;
  version: number;
  artifact_status: "ready" | "rejected";
  artifact_diagnostics: ProblemArtifactDiagnostic[];
  saved_at: ISODateTime;
}

export interface ProblemPublicationReceipt {
  problem_id: EntityID;
  revision_id?: EntityID;
  testset_revision_id?: EntityID;
  version: number;
  published_at?: ISODateTime;
  lifecycle: ProblemLifecycle;
}
