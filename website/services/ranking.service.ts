import { MOCK_TOP_CULTIVATORS, MOCK_RANKINGS } from "@/mock/rankings";
import type { Cultivator, Leaderboard, SpiritualRoot } from "@/types/ranking";

/**
 * Service for managing the Leaderboard (Bảng Phong Thần).
 * Simulated async behavior for future API integration.
 */
export const rankingService = {
  /**
   * Fetch full leaderboard data (by rating/realm).
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

  /**
   * Fetch ranking sorted by level/exp.
   */
  async getLevelRanking(): Promise<Cultivator[]> {
    await new Promise((resolve) => setTimeout(resolve, 800));
    return [...MOCK_RANKINGS]
      .sort((a, b) => (b.level ?? 0) - (a.level ?? 0))
      .map((c, i) => ({ ...c, rank: i + 1 }));
  },

  /**
   * Fetch ranking grouped by spiritual root.
   */
  async getSpiritualRootRanking(): Promise<Record<SpiritualRoot, Cultivator[]>> {
    await new Promise((resolve) => setTimeout(resolve, 800));
    const grouped: Partial<Record<SpiritualRoot, Cultivator[]>> = {};
    for (const c of MOCK_RANKINGS) {
      if (!c.spiritualRoot) continue;
      if (!grouped[c.spiritualRoot]) grouped[c.spiritualRoot] = [];
      grouped[c.spiritualRoot]!.push(c);
    }
    // Sort each group by rootPurity descending, re-rank within group
    for (const key of Object.keys(grouped) as SpiritualRoot[]) {
      grouped[key] = grouped[key]!
        .sort((a, b) => (b.rootPurity ?? 0) - (a.rootPurity ?? 0))
        .map((c, i) => ({ ...c, rank: i + 1 }));
    }
    return grouped as Record<SpiritualRoot, Cultivator[]>;
  },
};
