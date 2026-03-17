/**
 * Problem-related types mirroring backend problem DTOs.
 */

import type { Tag } from "./tag";

/** Difficulty levels for problems. */
export type Difficulty = "easy" | "medium" | "hard";

/** Problem entity returned by GET /problems/:id and POST /problems/find. */
export interface Problem {
  id: number;
  title: string;
  description: string;
  difficulty: Difficulty;
  time_limit_ms: number;
  memory_limit_kb: number;
  author_id: number;
  is_published: boolean;
  tags: Tag[] | null;
  created_at: string;
  updated_at: string;
}

/** POST /problems request body for creating a problem. */
export interface CreateProblemRequest {
  title: string;
  description: string;
  difficulty: Difficulty;
  time_limit_ms?: number;
  memory_limit_kb?: number;
  tag_ids?: number[];
}

/** PUT /problems/:id request body for updating a problem. */
export interface UpdateProblemRequest {
  title?: string;
  description?: string;
  difficulty?: Difficulty;
  time_limit_ms?: number;
  memory_limit_kb?: number;
  is_published?: boolean;
  tag_ids?: number[];
}
