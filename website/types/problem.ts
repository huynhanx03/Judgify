/**
 * Problem-related types mirroring backend problem DTOs.
 */

import type { Tag } from "./tag";
import type { DifficultyResponse } from "./difficulty";

/** Difficulty string key — used for local styling lookups. */
export type Difficulty = "easy" | "medium" | "hard";

/** Problem entity returned by GET /problems/:id and POST /problems/find. */
export interface Problem {
  id: number;
  title: string;
  description: string;
  difficulty_id: number;
  difficulty?: DifficultyResponse;
  time_limit_ms: number;
  memory_limit_kb: number;
  author_id: number;
  is_published: boolean;
  submission_count: number;
  accepted_count: number;
  acceptance_rate: number;
  is_solved?: boolean;
  tags: Tag[] | null;
  created_at: string;
  updated_at: string;
}
