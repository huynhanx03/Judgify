"use client";

/**
 * Ranking page — rating + level/EXP side by side (top 10).
 */

import { useEffect, useState } from "react";
import { LoadingSpinner } from "@/components/loading-spinner";
import { rankingService } from "@/services/ranking.service";
import type { Cultivator } from "@/types/ranking";
import { RankingHeroSection } from "@/modules/ranking/ranking-hero-section";
import { RankingTopList } from "@/modules/ranking/ranking-top-list";

interface RankingData {
  rating: Cultivator[];
  level: Cultivator[];
}

export default function RankingPage() {
  const [data, setData] = useState<RankingData | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    async function fetchAll() {
      try {
        const [rating, level] = await Promise.all([
          rankingService.getTopByRating(10),
          rankingService.getTopByExp(10),
        ]);
        setData({ rating, level });
      } catch (error) {
        console.error("Failed to fetch ranking data:", error);
      } finally {
        setIsLoading(false);
      }
    }
    fetchAll();
  }, []);

  if (isLoading || !data) return <LoadingSpinner />;

  return (
    <div className="space-y-12 pb-24">
      <RankingHeroSection />

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
    </div>
  );
}
