import { MOCK_TOP_CULTIVATORS, MOCK_RANKINGS } from "@/mock/rankings";
import type { Cultivator, Leaderboard } from "@/types/ranking";

/**
 * Service for managing the Leaderboard (Bảng Phong Thần).
 * Simulated async behavior for future API integration.
 */
export const rankingService = {
  /**
   * Fetch full leaderboard data.
   */
  async getLeaderboard(): Promise<Leaderboard> {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    return {
      topThree: MOCK_TOP_CULTIVATORS,
      others: MOCK_RANKINGS.filter(r => r.rank > 3),
    };
  },

  /**
   * Fetch top 3 cultivators only.
   */
  async getTopThree(): Promise<Cultivator[]> {
    await new Promise((resolve) => setTimeout(resolve, 500));
    return MOCK_TOP_CULTIVATORS;
  },
};
