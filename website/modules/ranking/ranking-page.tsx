"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { TEXT } from "@/constants/text";
import type { AsyncResourceStatus } from "@/hooks/use-retryable-resource";
import { RankingHeroSection } from "@/modules/ranking/ranking-hero-section";
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

function useRankingPage(kind: "rating" | "experience") {
  const [entries, setEntries] = useState<Cultivator[]>([]);
  const [nextCursor, setNextCursor] = useState<string>();
  const [status, setStatus] = useState<AsyncResourceStatus>("idle");
  const requestRef = useRef<AbortController | null>(null);

  const load = useCallback(async (cursor?: string) => {
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    setStatus("loading");
    try {
      const page = await fetchRankingPage(kind, cursor, controller.signal);
      if (controller.signal.aborted) return;
      setEntries((current) => cursor ? [...current, ...page.entries] : page.entries);
      setNextCursor(page.nextCursor);
      setStatus("ready");
    } catch {
      if (!controller.signal.aborted) setStatus("error");
    }
  }, [kind]);

  useEffect(() => {
    const controller = new AbortController();
    requestRef.current = controller;
    void fetchRankingPage(kind, undefined, controller.signal)
      .then((page) => {
        if (controller.signal.aborted) return;
        setEntries(page.entries);
        setNextCursor(page.nextCursor);
        setStatus("ready");
      })
      .catch(() => {
        if (!controller.signal.aborted) setStatus("error");
      });
    return () => controller.abort();
  }, [kind]);

  return {
    entries,
    status,
    hasMore: Boolean(nextCursor),
    retry: () => void load(),
    loadMore: () => {
      if (nextCursor && status !== "loading") void load(nextCursor);
    },
  };
}

export default function RankingPage() {
  const rating = useRankingPage("rating");
  const cultivation = useRankingPage("experience");

  return (
    <div className="space-y-12 pb-24">
      <RankingHeroSection />

      <div className="mx-auto grid max-w-7xl grid-cols-1 items-start gap-6 px-4 lg:grid-cols-2">
        <RankingTopList
          title={TEXT.RANKING.RATING_SECTION}
          subtitle={TEXT.RANKING.RATING_SECTION_SUBTITLE}
          icon="crown"
          accentColor="amber"
          data={rating.entries}
          mode="rating"
          status={rating.status}
          hasMore={rating.hasMore}
          onLoadMore={rating.loadMore}
          onRetry={rating.retry}
        />
        <RankingTopList
          title={TEXT.RANKING.CULTIVATION_SECTION}
          subtitle={TEXT.RANKING.CULTIVATION_SECTION_SUBTITLE}
          icon="flame"
          accentColor="emerald"
          data={cultivation.entries}
          mode="level"
          status={cultivation.status}
          hasMore={cultivation.hasMore}
          onLoadMore={cultivation.loadMore}
          onRetry={cultivation.retry}
        />
      </div>
    </div>
  );
}
