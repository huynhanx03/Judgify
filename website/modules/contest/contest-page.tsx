"use client";

/**
 * Contest page — list of contests with hero banner.
 * Server-backed contest discovery page.
 */

import { useMemo, useState } from "react";
import { TEXT } from "@/constants/text";
import { Search, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { PublicPageLead } from "@/components/public-page-lead";
import { PaginationControls } from "@/components/pagination-controls";
import { contestService } from "@/services/contest.service";
import {
  CONTEST_DISCOVERY,
  CONTEST_LIFECYCLE,
} from "@/constants/contest";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useContestLifecycleResource } from "@/hooks/use-contest-lifecycle-resource";
import { cn } from "@/lib/utils";
import { DataLoadingFeedback } from "@/modules/problem/data-load-feedback";
import { ResourceCollectionFeedback } from "@/modules/shared/resource-feedback";
import type { Contest, ContestStatus } from "@/types/contest";
import type { Paginated, QueryOptions, SearchFilter } from "@/types/api";
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
    () =>
      `${CONTEST_LIFECYCLE.PUBLIC_LIST_RESOURCE_KEY}:${JSON.stringify(query)}`,
    [query],
  );
  const contestResource =
    useContestLifecycleResource<Paginated<Contest> | null>({
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

  return (
    <div className="space-y-10 pb-16 sm:space-y-12 sm:pb-20">
      <PublicPageLead
        art="contest"
        eyebrow={TEXT.CONTEST.HERO_BADGE}
        title={TEXT.NAV.CONTEST}
        description={TEXT.CONTEST.HERO_DESCRIPTION}
      />

      <div className="space-y-8 sm:space-y-10">
        <div className="flex flex-col items-start justify-between gap-4 border-b border-border/40 pb-6 sm:flex-row sm:items-center">
          <h2 className="text-2xl font-bold tracking-tight sm:text-3xl">
            {TEXT.CONTEST.TITLE}
          </h2>
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
            bare
            label={TEXT.CONTEST.LIST_LOADING}
            className="min-h-64"
          />
        ) : contestResource.status === "error" ? (
          <ResourceCollectionFeedback
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
                <ContestCard key={contest.id} contest={contest} />
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
