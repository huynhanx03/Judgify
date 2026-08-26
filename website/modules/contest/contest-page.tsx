"use client";

/**
 * Contest page — list of contests with hero banner.
 * Server-backed contest discovery page.
 */

import { useMemo, useState } from "react";
import { TEXT } from "@/constants/text";
import { Trophy, Search, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { PaginationControls } from "@/components/pagination-controls";
import { contestService } from "@/services/contest.service";
import {
  CONTEST_DISCOVERY,
  CONTEST_LIFECYCLE,
  CONTEST_LIST_ANCHOR_ID,
} from "@/constants/contest";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useContestLifecycleResource } from "@/hooks/use-contest-lifecycle-resource";
import { cn } from "@/lib/utils";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import type { Contest, ContestStatus } from "@/types/contest";
import type { Paginated, QueryOptions, SearchFilter } from "@/types/api";
import { ContestHeroSection } from "@/modules/contest/contest-hero-section";
import { ContestCard } from "@/modules/contest/contest-card";

type StatusFilter = ContestStatus | "all";

const STATUS_FILTERS: { value: StatusFilter; label: string }[] = [
  { value: "all", label: TEXT.CONTEST.FILTER_ALL },
  { value: "running", label: TEXT.CONTEST.FILTER_RUNNING },
  { value: "upcoming", label: TEXT.CONTEST.FILTER_UPCOMING },
  { value: "ended", label: TEXT.CONTEST.FILTER_ENDED },
];

const EMPTY_CONTESTS: Paginated<Contest> | null = null;

export default function ContestPage() {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(
    search.trim(),
    CONTEST_DISCOVERY.SEARCH_DEBOUNCE_MS,
  );
  const [page, setPage] = useState(1);

  const query = useMemo<QueryOptions>(() => {
    const filters: SearchFilter[] = [];
    if (statusFilter !== "all") {
      filters.push({ key: "status", value: statusFilter, type: "exact" });
    }
    if (debouncedSearch) {
      filters.push({ key: "name", value: debouncedSearch, type: "search" });
    }
    return {
      pagination: { page, page_size: CONTEST_DISCOVERY.PAGE_SIZE },
      filters,
      sort: [{ key: "start_time", order: -1 }],
    };
  }, [debouncedSearch, page, statusFilter]);
  const queryKey = useMemo(
    () => `${CONTEST_LIFECYCLE.PUBLIC_LIST_RESOURCE_KEY}:${JSON.stringify(query)}`,
    [query],
  );
  const contestResource = useContestLifecycleResource<Paginated<Contest> | null>({
    resourceKey: queryKey,
    initialData: EMPTY_CONTESTS,
    load: (signal) => contestService.find(query, signal),
  });
  const contests = contestResource.data?.records ?? [];
  const pagination = contestResource.data?.pagination;
  const isInitialLoading =
    contestResource.status === "loading" && contestResource.data === null;
  const isRefreshing =
    contestResource.status === "loading" && contestResource.data !== null;
  const isSearchPending = search.trim() !== debouncedSearch;

  const selectStatus = (status: StatusFilter) => {
    setStatusFilter(status);
    setPage(1);
  };

  const focusContestList = (status: StatusFilter) => {
    selectStatus(status);
    window.requestAnimationFrame(() => {
      const reduceMotion = window.matchMedia(
        "(prefers-reduced-motion: reduce)",
      ).matches;
      document
        .getElementById(CONTEST_LIST_ANCHOR_ID)
        ?.scrollIntoView({
          behavior: reduceMotion ? "auto" : "smooth",
          block: "start",
        });
    });
  };

  return (
    <div className="space-y-12 pb-20">
      <ContestHeroSection
        onExplore={() => focusContestList("all")}
        onShowUpcoming={() => focusContestList("upcoming")}
      />

      <div
        id={CONTEST_LIST_ANCHOR_ID}
        className="mx-auto max-w-6xl scroll-mt-24 space-y-10 px-4"
      >
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between border-b border-border/40 pb-6 gap-4">
          <div className="flex items-center gap-3">
            <Trophy className="h-8 w-8 text-cultivation" aria-hidden="true" />
            <h2 className="text-3xl font-black tracking-tight">{TEXT.CONTEST.TITLE}</h2>
          </div>
          <div className="flex flex-wrap gap-2">
            {STATUS_FILTERS.map((f) => (
              <button
                type="button"
                key={f.value}
                aria-pressed={statusFilter === f.value}
                className={
                  statusFilter === f.value
                    ? "min-h-11 rounded-full border border-cultivation/20 bg-cultivation/10 px-4 py-1.5 text-sm font-bold text-cultivation outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 motion-reduce:transition-none"
                    : "min-h-11 rounded-full border border-border/60 px-4 py-1.5 text-sm font-bold text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
                }
                onClick={() => selectStatus(f.value)}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>

        <div className="relative max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="search"
            aria-label={TEXT.CONTEST.SEARCH_LABEL}
            placeholder={TEXT.CONTEST.SEARCH_PLACEHOLDER}
            value={search}
            maxLength={CONTEST_DISCOVERY.MAXIMUM_SEARCH_CHARACTERS}
            onChange={(event) => {
              setSearch(event.target.value);
              setPage(1);
            }}
            className="min-h-11 rounded-full pl-10 pr-10"
          />
          {isSearchPending || isRefreshing ? (
            <Loader2
              className="absolute right-3 top-1/2 size-4 -translate-y-1/2 animate-spin text-muted-foreground motion-reduce:animate-none"
              aria-label={TEXT.CONTEST.REFRESHING}
            />
          ) : null}
        </div>

        {isInitialLoading ? (
          <DataLoadingFeedback
            label={TEXT.CONTEST.LIST_LOADING}
            className="min-h-64"
          />
        ) : contestResource.status === "error" ? (
          <DataLoadFeedback
            title={TEXT.CONTEST.LOAD_ERROR_TITLE}
            description={TEXT.CONTEST.LOAD_ERROR_DESCRIPTION}
            retryLabel={TEXT.CONTEST.RETRY}
            onRetry={contestResource.retry}
          />
        ) : contests.length > 0 ? (
          <div
            className={cn(
              "space-y-8 transition-opacity",
              isRefreshing && "opacity-60",
            )}
            aria-busy={isRefreshing}
          >
            <div className="grid grid-cols-1 gap-8">
              {contests.map((contest) => (
                <ContestCard
                  key={contest.id}
                  contest={contest}
                />
              ))}
            </div>
            {pagination ? (
              <PaginationControls
                pagination={pagination}
                onPageChange={setPage}
              />
            ) : null}
          </div>
        ) : (
          <div className="text-center py-20 text-muted-foreground">
            <p className="text-xl font-bold">{TEXT.CONTEST.NO_CONTESTS}</p>
            <p className="text-sm mt-2">
              {search.trim() || statusFilter !== "all"
                ? TEXT.CONTEST.NO_MATCHING_CONTESTS_DESC
                : TEXT.CONTEST.NO_CONTESTS_DESC}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
