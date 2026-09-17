/**
 * Ranking-related types for the Leaderboard (Bảng Phong Thần).
 */

import type { EntityID } from "@/types/api";

export interface Cultivator {
  id: EntityID;
  rank: number;
  name: string;
	displayMode: "public" | "anonymous";
  realm?: string;
  points: number;
  avatar?: string;
  color?: string;
  /** Level of the cultivator */
  level?: number;
  /** Experience points */
  exp?: number;
  /** Max experience for current level */
  maxExp?: number;
}

export interface RankingEntryPayload {
	rank: number;
	public_id: EntityID;
	display_name?: string;
	display_mode: "public" | "anonymous";
	rating: number;
	rank_title?: string;
	total_exp: number;
	level_name?: string;
	level?: number;
}

export interface RankingPagePayload {
	entries: RankingEntryPayload[];
	next_cursor?: string;
	projection_revision: number;
}

export interface RankingPage {
	entries: Cultivator[];
	nextCursor?: string;
	projectionRevision: number;
}

export type RankingVisibility = "public" | "anonymous" | "hidden";

export interface RankingPrivacy {
	visibility: RankingVisibility;
	version: number;
	projection_revision: number;
	replayed?: boolean;
}

export interface Leaderboard {
  topThree: Cultivator[];
  others: Cultivator[];
}
