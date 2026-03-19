"use client";

/**
 * Ranking page — single-page layout with:
 * - Hero section
 * - Rating + Level/EXP side by side (top 10, equal height)
 * - 5 spiritual root columns (Kim Mộc Thủy Hỏa Thổ)
 */

import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { rankingService } from "@/services/ranking.service";
import type { Cultivator } from "@/types/ranking";
import { RankingHeroSection } from "@/modules/ranking/ranking-hero-section";
import { RankingTopList } from "@/modules/ranking/ranking-top-list";
// TODO: Enable when spiritual root ranking is ready
// import { RankingSpiritualRootGrid } from "@/modules/ranking/ranking-spiritual-root-grid";

interface RankingData {
  rating: Cultivator[];
  level: Cultivator[];
}

export default function RankingPage() {
  const [data, setData] = useState<RankingData | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchAll = async () => {
      try {
        const [leaderboard, level] = await Promise.all([
          rankingService.getLeaderboard(),
          rankingService.getLevelRanking(),
        ]);
        setData({
          rating: [...leaderboard.topThree, ...leaderboard.others].slice(0, 10),
          level: level.slice(0, 10),
        });
      } catch (error) {
        console.error("Failed to fetch ranking data:", error);
      } finally {
        setIsLoading(false);
      }
    };
    fetchAll();
  }, []);

  if (isLoading || !data) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Loader2 className="h-10 w-10 text-primary animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-12 pb-24">
      <RankingHeroSection />

      {/* Rating + Level side by side, equal height */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 max-w-7xl mx-auto px-4 items-stretch">
        <RankingTopList
          title="Thiên Đạo Bảng"
          subtitle="Cảnh giới tu luyện"
          icon="crown"
          accentColor="amber"
          data={data.rating}
          mode="rating"
        />
        <RankingTopList
          title="Tu Luyện Bảng"
          subtitle="Cấp độ & danh hiệu"
          icon="flame"
          accentColor="emerald"
          data={data.level}
          mode="level"
        />
      </div>

      {/* TODO: Enable when spiritual root ranking is ready */}
      {/* <RankingSpiritualRootGrid data={data.roots} /> */}
    </div>
  );
}
