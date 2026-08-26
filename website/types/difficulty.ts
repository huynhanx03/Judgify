/**
 * Difficulty-related types mirroring backend difficulty DTOs.
 */

import type { EntityID } from "@/types/api";

/** Difficulty entity — mirrors BE DifficultyResponse. */
export interface DifficultyResponse {
  id: EntityID;
  name: string;
  level: number;
  exp_reward: number;
  description?: string;
  version: number;
}

/** Map difficulty level to a slug for styling lookups. */
export const DIFFICULTY_SLUG: Record<number, string> = {
  1: "easy",
  2: "medium",
  3: "hard",
};
