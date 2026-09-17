"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  ARENA_POLICY,
  ARENA_QUERY_PARAMETER,
  ARENA_SORT_DIRECTION,
  ARENA_SORT_FIELD,
  type ArenaSortDirection,
  type ArenaSortField,
} from "@/constants/arena";
import { tryEntityID } from "@/lib/api/contracts";

export interface UseArenaFiltersReturn {
  searchTerm: string;
  debouncedSearch: string;
  difficultyFilter: string | null;
  tagFilters: string[];
  tagLimitReached: boolean;
  sortField: ArenaSortField;
  sortDirection: ArenaSortDirection;
  currentPage: number;
  setSearchTerm: (value: string) => void;
  setDifficultyFilter: (value: string | null) => void;
  toggleTag: (id: string) => void;
  removeTag: (id: string) => void;
  clearTags: () => void;
  setSort: (field: ArenaSortField) => void;
  setSortDirection: (direction: ArenaSortDirection) => void;
  setPage: (page: number) => void;
  clearAll: () => void;
  reconcileCatalogs: (
    difficultyIDs: ReadonlySet<string>,
    tagIDs: ReadonlySet<string>,
  ) => void;
  hasActiveFilters: boolean;
}

function boundedSearch(value: string | null): string {
  return (value ?? "")
    .slice(0, ARENA_POLICY.MAXIMUM_SEARCH_CHARACTERS);
}

function readEntityID(value: string | null): string | null {
  return tryEntityID(value);
}

function readTagFilters(searchParams: URLSearchParams): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const value of searchParams.getAll(ARENA_QUERY_PARAMETER.TAG)) {
    const id = readEntityID(value);
    if (!id || seen.has(id)) continue;
    seen.add(id);
    result.push(id);
    if (result.length === ARENA_POLICY.MAXIMUM_TAG_FILTERS) break;
  }
  return result;
}

function readSortField(value: string | null): ArenaSortField {
  return value === ARENA_SORT_FIELD.ACCEPTANCE ||
      value === ARENA_SORT_FIELD.SUBMISSIONS
    ? value
    : ARENA_SORT_FIELD.NONE;
}

function readSortDirection(value: string | null): ArenaSortDirection {
  return value === ARENA_SORT_DIRECTION.ASCENDING
    ? ARENA_SORT_DIRECTION.ASCENDING
    : ARENA_SORT_DIRECTION.DESCENDING;
}

function readPage(value: string | null): number {
  if (!value || !/^[1-9]\d*$/.test(value)) return 1;
  const page = Number(value);
  return Number.isSafeInteger(page) &&
      page <= ARENA_POLICY.MAXIMUM_PAGE
    ? page
    : 1;
}

/**
 * URL-backed arena filters. A refresh, shared link, or browser back operation
 * restores the same query without introducing a second persistence store.
 */
export function useArenaFilters(): UseArenaFiltersReturn {
  const router = useRouter();
  const pathname = usePathname();
  const readonlySearchParams = useSearchParams();
  const serializedSearchParams = readonlySearchParams.toString();
  const searchParams = useMemo(
    () => new URLSearchParams(serializedSearchParams),
    [serializedSearchParams],
  );

  const urlSearch = boundedSearch(
    searchParams.get(ARENA_QUERY_PARAMETER.SEARCH),
  );
  const [searchDraft, setSearchDraft] = useState<{
    source: string;
    value: string;
  } | null>(null);
  const [debouncedDraft, setDebouncedDraft] = useState<{
    source: string;
    value: string;
  } | null>(null);
  const searchTerm = searchDraft?.source === urlSearch
    ? searchDraft.value
    : urlSearch;
  const debouncedSearch = debouncedDraft?.source === urlSearch
    ? debouncedDraft.value
    : urlSearch.trim();

  const difficultyFilter = readEntityID(
    searchParams.get(ARENA_QUERY_PARAMETER.DIFFICULTY),
  );
  const tagFilters = useMemo(
    () => readTagFilters(searchParams),
    [searchParams],
  );
  const sortField = readSortField(
    searchParams.get(ARENA_QUERY_PARAMETER.SORT),
  );
  const sortDirection = readSortDirection(
    searchParams.get(ARENA_QUERY_PARAMETER.DIRECTION),
  );
  const currentPage = readPage(
    searchParams.get(ARENA_QUERY_PARAMETER.PAGE),
  );

  const replaceSearchParams = useCallback(
    (mutate: (next: URLSearchParams) => void) => {
      const next = new URLSearchParams(serializedSearchParams);
      mutate(next);
      const serialized = next.toString();
      if (serialized === serializedSearchParams) return;
      router.replace(serialized ? `${pathname}?${serialized}` : pathname, {
        scroll: false,
      });
    },
    [pathname, router, serializedSearchParams],
  );

  useEffect(() => {
    const timeout = setTimeout(() => {
      const value = searchTerm.trim();
      setDebouncedDraft({ source: urlSearch, value });
      replaceSearchParams((next) => {
        if (value) next.set(ARENA_QUERY_PARAMETER.SEARCH, value);
        else next.delete(ARENA_QUERY_PARAMETER.SEARCH);
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    }, ARENA_POLICY.SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timeout);
  }, [replaceSearchParams, searchTerm, urlSearch]);

  const setSearchTerm = useCallback((value: string) => {
    setSearchDraft({
      source: urlSearch,
      value: value.slice(0, ARENA_POLICY.MAXIMUM_SEARCH_CHARACTERS),
    });
  }, [setSearchDraft, urlSearch]);

  const setDifficultyFilter = useCallback(
    (value: string | null) => {
      const id = value === null ? null : readEntityID(value);
      replaceSearchParams((next) => {
        if (id) next.set(ARENA_QUERY_PARAMETER.DIFFICULTY, id);
        else next.delete(ARENA_QUERY_PARAMETER.DIFFICULTY);
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [replaceSearchParams],
  );

  const toggleTag = useCallback(
    (value: string) => {
      const id = readEntityID(value);
      if (!id) return;
      const selected = readTagFilters(searchParams);
      const nextTags = selected.includes(id)
        ? selected.filter((tagID) => tagID !== id)
        : selected.length < ARENA_POLICY.MAXIMUM_TAG_FILTERS
          ? [...selected, id]
          : selected;
      replaceSearchParams((next) => {
        next.delete(ARENA_QUERY_PARAMETER.TAG);
        nextTags.forEach((tagID) =>
          next.append(ARENA_QUERY_PARAMETER.TAG, tagID),
        );
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [replaceSearchParams, searchParams],
  );

  const removeTag = useCallback(
    (id: string) => {
      const nextTags = tagFilters.filter((tagID) => tagID !== id);
      replaceSearchParams((next) => {
        next.delete(ARENA_QUERY_PARAMETER.TAG);
        nextTags.forEach((tagID) =>
          next.append(ARENA_QUERY_PARAMETER.TAG, tagID),
        );
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [replaceSearchParams, tagFilters],
  );

  const clearTags = useCallback(() => {
    replaceSearchParams((next) => {
      next.delete(ARENA_QUERY_PARAMETER.TAG);
      next.delete(ARENA_QUERY_PARAMETER.PAGE);
    });
  }, [replaceSearchParams]);

  const setSort = useCallback(
    (field: ArenaSortField) => {
      replaceSearchParams((next) => {
        if (field === ARENA_SORT_FIELD.NONE) {
          next.delete(ARENA_QUERY_PARAMETER.SORT);
          next.delete(ARENA_QUERY_PARAMETER.DIRECTION);
        } else {
          const toggledDirection =
            sortField === field
              ? sortDirection === ARENA_SORT_DIRECTION.DESCENDING
                ? ARENA_SORT_DIRECTION.ASCENDING
                : ARENA_SORT_DIRECTION.DESCENDING
              : ARENA_SORT_DIRECTION.DESCENDING;
          next.set(ARENA_QUERY_PARAMETER.SORT, field);
          next.set(ARENA_QUERY_PARAMETER.DIRECTION, toggledDirection);
        }
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [replaceSearchParams, sortDirection, sortField],
  );

  const setSortDirection = useCallback(
    (direction: ArenaSortDirection) => {
      if (sortField === ARENA_SORT_FIELD.NONE) return;
      replaceSearchParams((next) => {
        next.set(ARENA_QUERY_PARAMETER.DIRECTION, direction);
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [replaceSearchParams, sortField],
  );

  const setPage = useCallback(
    (page: number) => {
      const bounded = Math.min(
        ARENA_POLICY.MAXIMUM_PAGE,
        Math.max(1, Math.trunc(page)),
      );
      replaceSearchParams((next) => {
        if (bounded === 1) next.delete(ARENA_QUERY_PARAMETER.PAGE);
        else next.set(ARENA_QUERY_PARAMETER.PAGE, String(bounded));
      });
    },
    [replaceSearchParams],
  );

  const clearAll = useCallback(() => {
    setSearchDraft({ source: urlSearch, value: "" });
    setDebouncedDraft({ source: urlSearch, value: "" });
    replaceSearchParams((next) => {
      Object.values(ARENA_QUERY_PARAMETER).forEach((key) =>
        next.delete(key),
      );
    });
  }, [
    replaceSearchParams,
    setDebouncedDraft,
    setSearchDraft,
    urlSearch,
  ]);

  const reconcileCatalogs = useCallback(
    (
      difficultyIDs: ReadonlySet<string>,
      tagIDs: ReadonlySet<string>,
    ) => {
      const validDifficulty =
        difficultyFilter && difficultyIDs.has(difficultyFilter)
          ? difficultyFilter
          : null;
      const validTags = tagFilters.filter((id) => tagIDs.has(id));
      if (
        validDifficulty === difficultyFilter &&
        validTags.length === tagFilters.length
      ) {
        return;
      }
      replaceSearchParams((next) => {
        if (validDifficulty) {
          next.set(ARENA_QUERY_PARAMETER.DIFFICULTY, validDifficulty);
        } else {
          next.delete(ARENA_QUERY_PARAMETER.DIFFICULTY);
        }
        next.delete(ARENA_QUERY_PARAMETER.TAG);
        validTags.forEach((id) =>
          next.append(ARENA_QUERY_PARAMETER.TAG, id),
        );
        next.delete(ARENA_QUERY_PARAMETER.PAGE);
      });
    },
    [
      difficultyFilter,
      replaceSearchParams,
      tagFilters,
    ],
  );

  const hasActiveFilters =
    difficultyFilter !== null ||
    tagFilters.length > 0 ||
    sortField !== ARENA_SORT_FIELD.NONE ||
    searchTerm.trim() !== "";

  return {
    searchTerm,
    debouncedSearch,
    difficultyFilter,
    tagFilters,
    tagLimitReached:
      tagFilters.length >= ARENA_POLICY.MAXIMUM_TAG_FILTERS,
    sortField,
    sortDirection,
    currentPage,
    setSearchTerm,
    setDifficultyFilter,
    toggleTag,
    removeTag,
    clearTags,
    setSort,
    setSortDirection,
    setPage,
    clearAll,
    reconcileCatalogs,
    hasActiveFilters,
  };
}
