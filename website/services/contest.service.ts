import { MOCK_CONTESTS } from "@/mock/contests";
import type { Contest } from "@/types/contest";

/**
 * Service for managing Great Arena Assembly (Đại Hội Tỷ Thí).
 * Simulated async behavior for future API integration.
 */
export const contestService = {
  /**
   * Fetch all contests.
   */
  async getContests(): Promise<Contest[]> {
    await new Promise((resolve) => setTimeout(resolve, 800));
    return MOCK_CONTESTS;
  },

  /**
   * Fetch a single contest by ID.
   */
  async getContestById(id: number): Promise<Contest | undefined> {
    await new Promise((resolve) => setTimeout(resolve, 400));
    return MOCK_CONTESTS.find((c) => c.id === id);
  },
};
