"use client";

import { useEffect, useMemo } from "react";
import { Loader2, Swords } from "lucide-react";
import { PaginationControls } from "@/components/pagination-controls";
import {
  ARENA_POLICY,
  ARENA_SORT_DIRECTION,
  ARENA_SORT_FIELD,
} from "@/constants/arena";
import { TEXT } from "@/constants/text";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { ArenaFilterBar } from "@/modules/arena/components/arena-filter-bar";
import { ProblemTable } from "@/modules/arena/components/problem-table";
import { useArenaFilters } from "@/modules/arena/hooks/use-arena-filters";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import { difficultyService } from "@/services/difficulty.service";
import { problemService } from "@/services/problem.service";
import { tagService } from "@/services/tag.service";
import type { Paginated, QueryOptions } from "@/types/api";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Problem } from "@/types/problem";
import type { Tag } from "@/types/tag";

const EMPTY_TAGS: Tag[] = [];
const EMPTY_DIFFICULTIES: DifficultyResponse[] = [];

export function ArenaPage() {
  const filters = useArenaFilters();
  const {
    debouncedSearch,
    difficultyFilter,
    tagFilters,
    sortField,
    sortDirection,
    currentPage,
    setPage,
    reconcileCatalogs,
  } = filters;

  const tagsResource = useRetryableResource<Tag[]>({
    resetKey: "arena-tags",
    initialData: EMPTY_TAGS,
    load: tagService.getAll,
  });
  const difficultiesResource = useRetryableResource<DifficultyResponse[]>({
    resetKey: "arena-difficulties",
    initialData: EMPTY_DIFFICULTIES,
    load: difficultyService.getAll,
  });

  const query = useMemo<QueryOptions>(() => {
    const queryFilters: NonNullable<QueryOptions["filters"]> = [];
    if (debouncedSearch) {
      queryFilters.push({
        key: "title",
        value: debouncedSearch,
        type: "search",
      });
    }
    if (difficultyFilter) {
      queryFilters.push({
        key: "difficulty_id",
        value: difficultyFilter,
        type: "filter",
      });
    }
    if (tagFilters.length > 0) {
      queryFilters.push({
        key: "tag_ids",
        value: tagFilters,
        type: "filter",
      });
    }

    const sort: NonNullable<QueryOptions["sort"]> = [];
    if (sortField === ARENA_SORT_FIELD.ACCEPTANCE) {
      sort.push({
        key: "acceptance_rate",
        order:
          sortDirection === ARENA_SORT_DIRECTION.ASCENDING ? 1 : -1,
      });
    } else if (sortField === ARENA_SORT_FIELD.SUBMISSIONS) {
      sort.push({
        key: "submission_count",
        order:
          sortDirection === ARENA_SORT_DIRECTION.ASCENDING ? 1 : -1,
      });
      sort.push({ key: "id", order: -1 });
    }

    return {
      pagination: {
        page: currentPage,
        page_size: ARENA_POLICY.PAGE_SIZE,
      },
      filters: queryFilters.length > 0 ? queryFilters : undefined,
      sort: sort.length > 0 ? sort : undefined,
    };
  }, [
    currentPage,
    debouncedSearch,
    difficultyFilter,
    sortDirection,
    sortField,
    tagFilters,
  ]);
  const queryKey = useMemo(() => JSON.stringify(query), [query]);
  const problemsResource = useRetryableResource<Paginated<Problem> | null>({
    resetKey: queryKey,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => problemService.find(query, signal),
  });

  useEffect(() => {
    if (
      tagsResource.status !== "ready" ||
      difficultiesResource.status !== "ready"
    ) {
      return;
    }
    reconcileCatalogs(
      new Set(
        difficultiesResource.data.map((difficulty) => difficulty.id),
      ),
      new Set(tagsResource.data.map((tag) => tag.id)),
    );
  }, [
    difficultiesResource.data,
    difficultiesResource.status,
    reconcileCatalogs,
    tagsResource.data,
    tagsResource.status,
  ]);

  const referenceDataLoading =
    tagsResource.status === "loading" ||
    difficultiesResource.status === "loading";
  const referenceDataError =
    tagsResource.status === "error" ||
    difficultiesResource.status === "error";
  const hasData = problemsResource.data !== null;
  const canRenderPreviousData =
    problemsResource.status === "loading" && hasData;
  const hasCurrentData =
    hasData && !problemsResource.isPreviousData;
  const shouldRenderTable = hasCurrentData || canRenderPreviousData;
  const problems = shouldRenderTable
    ? (problemsResource.data?.records ?? [])
    : [];
  const pagination = hasCurrentData
    ? problemsResource.data?.pagination
    : undefined;
  const isRefreshing =
    problemsResource.status === "loading" && shouldRenderTable;

  function retryReferenceData() {
    if (tagsResource.status === "error") tagsResource.retry();
    if (difficultiesResource.status === "error") {
      difficultiesResource.retry();
    }
  }

  return (
    <div className="space-y-6 motion-safe:animate-in motion-safe:fade-in">
      <header>
        <div className="flex items-center gap-3">
          <span className="flex size-11 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-primary shadow-brand-soft">
            <Swords className="size-6" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h1 className="font-heading text-2xl font-bold tracking-tight text-foreground sm:text-3xl lg:text-4xl">
              {TEXT.ARENA.TITLE}
            </h1>
            <p className="mt-1 text-sm font-medium leading-6 text-muted-foreground">
              {TEXT.ARENA.SUBTITLE}
            </p>
          </div>
        </div>
      </header>

      <ArenaFilterBar
        filters={filters}
        tags={tagsResource.data}
        difficulties={difficultiesResource.data}
        referenceDataLoading={referenceDataLoading}
      />

      {referenceDataError ? (
        <DataLoadFeedback
          compact
          title={TEXT.ARENA.FILTERS_LOAD_ERROR_TITLE}
          description={TEXT.ARENA.FILTERS_LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.ARENA.RETRY}
          onRetry={retryReferenceData}
        />
      ) : null}

      <section
        className="space-y-4"
        aria-busy={problemsResource.status === "loading"}
        aria-labelledby="arena-results-heading"
      >
        <div className="flex min-h-6 items-center justify-between gap-3 px-1">
          <h2
            id="arena-results-heading"
            className="text-sm font-semibold text-foreground"
          >
            {pagination
              ? TEXT.ARENA.RESULTS_SUMMARY(pagination.total_items)
              : TEXT.ARENA.PROBLEM_TITLE}
          </h2>
          {isRefreshing ? (
            <p
              className="flex items-center gap-2 text-xs text-muted-foreground"
              role="status"
              aria-live="polite"
            >
              <Loader2
                className="size-3.5 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
              {TEXT.ARENA.PROBLEMS_REFRESHING}
            </p>
          ) : null}
        </div>

        {problemsResource.status === "loading" && !shouldRenderTable ? (
          <DataLoadingFeedback
            label={TEXT.ARENA.PROBLEMS_LOADING}
            className="min-h-72"
          />
        ) : problemsResource.status === "error" && !hasCurrentData ? (
          <DataLoadFeedback
            title={TEXT.ARENA.LOAD_ERROR_TITLE}
            description={TEXT.ARENA.LOAD_ERROR_DESCRIPTION}
            retryLabel={TEXT.ARENA.RETRY}
            onRetry={problemsResource.retry}
          />
        ) : (
          <>
            {problemsResource.status === "error" ? (
              <DataLoadFeedback
                compact
                title={TEXT.ARENA.LOAD_ERROR_TITLE}
                description={TEXT.ARENA.LOAD_ERROR_DESCRIPTION}
                retryLabel={TEXT.ARENA.RETRY}
                onRetry={problemsResource.retry}
              />
            ) : null}
            <ProblemTable
              problems={problems}
              sortField={filters.sortField}
              sortDirection={filters.sortDirection}
              onSort={filters.setSort}
              hasActiveFilters={filters.hasActiveFilters}
              onClearFilters={filters.clearAll}
            />
          </>
        )}

        {pagination ? (
          <div className="pb-6 pt-1">
            <PaginationControls
              pagination={pagination}
              onPageChange={setPage}
            />
          </div>
        ) : null}
      </section>
    </div>
  );
}
