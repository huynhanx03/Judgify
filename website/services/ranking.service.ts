import { apiClient } from "@/lib/api-client";
import { RANKING_API } from "@/constants/api";
import type { Cultivator } from "@/types/ranking";

interface RankingEntry {
  rank: number;
  user_id: number;
  username: string;
  rating: number;
  rank_title: string;
  total_exp: number;
  level_name: string;
  level: number;
}

function mapRatingEntry(e: RankingEntry): Cultivator {
  return {
    rank: e.rank,
    name: e.username,
    realm: e.rank_title || "Phàm Nhân",
    sect: "",
    points: e.rating,
  };
}

function mapLevelEntry(e: RankingEntry): Cultivator {
  return {
    rank: e.rank,
    name: e.username,
    realm: e.level_name || "Tạp Dịch Đệ Tử",
    sect: e.level_name || "",
    level: e.level,
    exp: e.total_exp,
    points: e.total_exp,
  };
}

export const rankingService = {
  async getTopByRating(limit = 10): Promise<Cultivator[]> {
    const entries = await apiClient.get<RankingEntry[]>(
      RANKING_API.TOP_RATING(limit)
    );
    return entries.map(mapRatingEntry);
  },
  async getTopByExp(limit = 10): Promise<Cultivator[]> {
    const entries = await apiClient.get<RankingEntry[]>(
      RANKING_API.TOP_EXP(limit)
    );
    return entries.map(mapLevelEntry);
  },
};
