/**
 * Ranking-related types for the Leaderboard (Bảng Phong Thần).
 */

export interface Cultivator {
  rank: number;
  name: string;
  realm: string;
  sect: string;
  points: number;
  avatar?: string;
  color?: string;
}

export interface Leaderboard {
  topThree: Cultivator[];
  others: Cultivator[];
}
