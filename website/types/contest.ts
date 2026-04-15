/**
 * Contest-related types mirroring backend contest DTOs.
 */

/** Contest status lifecycle. */
export type ContestStatus = "draft" | "upcoming" | "running" | "ended";

/** Contest entity returned by GET /contests/:id and POST /contests/find. */
export interface Contest {
  id: number;
  title: string;
  description: string;
  start_time: string;
  end_time: string;
  status: ContestStatus;
  author_id: number;
  max_participants: number;
  participant_count: number;
  problem_ids: number[];
  is_registered?: boolean;
  created_at: string;
}

/** Standing row returned by GET /contests/:id/standings. */
export interface Standing {
  rank: number;
  user_id: number;
  username: string;
  solved_count: number;
  penalty: number;
  problem_results: Record<string, unknown>;
}

/** Rating change row returned by GET /contests/:id/rating-changes. */
export interface RatingChange {
  user_id: number;
  username: string;
  old_rating: number;
  new_rating: number;
  rank: number;
  delta: number;
}
