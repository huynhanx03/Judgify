"use client";

import { useCallback } from "react";
import { TEXT } from "@/constants/text";
import { PublicPageLead } from "@/components/public-page-lead";
import { useCursorFeed } from "@/hooks/use-cursor-feed";
import { RankingTopList } from "@/modules/ranking/ranking-top-list";
import { rankingService } from "@/services/ranking.service";
import type { Cultivator, RankingPage } from "@/types/ranking";

const RANKING_PAGE_SIZE = 20;

function fetchRankingPage(
  kind: "rating" | "experience",
  cursor: string | undefined,
  signal: AbortSignal,
): Promise<RankingPage> {
  return kind === "rating"
    ? rankingService.pageByRating(RANKING_PAGE_SIZE, cursor, signal)
    : rankingService.pageByExperience(RANKING_PAGE_SIZE, cursor, signal);
}

const rankingEntryKey = (entry: Cultivator) => entry.id;

function useRankingPage(kind: "rating" | "experience") {
  const load = useCallback(
    async (cursor: string | undefined, signal: AbortSignal) => {
      const page = await fetchRankingPage(kind, cursor, signal);
      return { items: page.entries, next_cursor: page.nextCursor };
    },
    [kind],
  );

  return useCursorFeed({
    resetKey: kind,
    load,
    keyOf: rankingEntryKey,
  });
}

export default function RankingPage() {
  const rating = useRankingPage("rating");
  const cultivation = useRankingPage("experience");

  return (
    <div className="space-y-10 pb-16 sm:space-y-12 sm:pb-20">
      <PublicPageLead
        art="ranking"
        eyebrow={TEXT.RANKING.HERO_BADGE}
        title={TEXT.NAV.RANKING}
        description={TEXT.RANKING.HERO_DESC}
      />

      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-2">
        <RankingTopList
          title={TEXT.RANKING.RATING_SECTION}
          subtitle={TEXT.RANKING.RATING_SECTION_SUBTITLE}
          icon="crown"
          accentColor="amber"
          data={rating.items}
          mode="rating"
          status={rating.status}
          hasMore={rating.hasMore}
          onLoadMore={rating.loadMore}
          onRetry={rating.reload}
        />
        <RankingTopList
          title={TEXT.RANKING.CULTIVATION_SECTION}
          subtitle={TEXT.RANKING.CULTIVATION_SECTION_SUBTITLE}
          icon="flame"
          accentColor="emerald"
          data={cultivation.items}
          mode="level"
          status={cultivation.status}
          hasMore={cultivation.hasMore}
          onLoadMore={cultivation.loadMore}
          onRetry={cultivation.reload}
        />
      </div>
    </div>
  );
}
