/**
 * User profile types — mirrors api/internal/identity/core/dto/user.go FullProfileResponse.
 */

export interface TraitInfo {
  name: string
  rarity_code?: string
  rarity_name?: string
  description?: string
  metadata?: Record<string, unknown>
}

export interface ElementExp {
  code: string
  name: string
  exp: number
}

export interface LevelInfo {
  name: string
  next_name?: string        // empty if at max level
  tier_index: number        // 0-based index for color palette
  next_tier_index?: number  // tier_index of next level; absent if at max
  progress: number          // 0–100
  exp_to_next: number
}

export interface RankInfo {
  name: string
  next_name?: string        // empty if at max rank
  tier_index: number        // 0-based index for color palette
  next_tier_index?: number  // tier_index of next rank; absent if at max
  progress: number          // 0–100
  rating_to_next: number
}

export interface CultivationInfo {
  total_exp: number
  rating: number
  level: LevelInfo
  rank: RankInfo
  root_bone?: TraitInfo
  talents: TraitInfo[]
  elements: ElementExp[]
}

export interface ElementBrief {
  code: string
  name: string
}

export interface DiffStat {
  name: string
  level: number
  solved_count: number
}

export interface TagStat {
  name: string
  solved_count: number
  elements: ElementBrief[]
}

export interface ProblemStats {
  total_submissions: number
  accepted_count: number
  by_difficulty: DiffStat[]
  by_tag: TagStat[]
}

export interface UserProfile {
  username: string
  first_name: string
  last_name: string
  gender: number
  birthday?: string
  joined_at: string
  cultivation: CultivationInfo
  problem_stats: ProblemStats
}

export interface UpdateProfileRequest {
  first_name: string
  last_name: string
  gender: number
  birthday: string
}

/** Create a minimal UserProfile for instant display after login (before full profile loads). */
export function createDefaultProfile(username: string): UserProfile {
  return {
    username,
    first_name: "",
    last_name: "",
    gender: 0,
    joined_at: "",
    cultivation: {
      total_exp: 0,
      rating: 0,
      level: { name: "—", tier_index: 0, progress: 0, exp_to_next: 0 },
      rank: { name: "—", tier_index: 0, progress: 0, rating_to_next: 0 },
      talents: [],
      elements: [],
    },
    problem_stats: {
      total_submissions: 0,
      accepted_count: 0,
      by_difficulty: [],
      by_tag: [],
    },
  };
}
