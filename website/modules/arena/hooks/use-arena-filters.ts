"use client";

/**
 * Manages all filter/sort/search/pagination state for the Arena page.
 * Debounces search (400ms). Resets page to 1 on any filter change.
 * Fixes the missing cleanup-on-unmount memory leak from the original debounce.
 */

import { useState, useEffect, useRef } from "react";
import type { SortField, SortDirection } from "@/modules/arena/components/problem-table";

export type Difficulty = "easy" | "medium" | "hard";

export interface UseArenaFiltersReturn {
  searchTerm: string;
  debouncedSearch: string;
  difficultyFilter: Difficulty | "all";
  tagFilters: number[];
  sortField: SortField;
  sortDirection: SortDirection;
  currentPage: number;
  setSearchTerm: (v: string) => void;
  setDifficultyFilter: (v: Difficulty | "all") => void;
  toggleTag: (id: number) => void;
  removeTag: (id: number) => void;
  setSort: (field: SortField) => void;
  setPage: (p: number) => void;
  clearAll: () => void;
  hasActiveFilters: boolean;
}

export function useArenaFilters(): UseArenaFiltersReturn {
  const [searchTerm, setSearchTermRaw] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [difficultyFilter, setDifficultyFilterRaw] = useState<Difficulty | "all">("all");
  const [tagFilters, setTagFilters] = useState<number[]>([]);
  const [sortField, setSortField] = useState<SortField>("none");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const [currentPage, setCurrentPage] = useState(1);

  const debounceRef = useRef<NodeJS.Timeout | null>(null);

  // Debounce search with cleanup on unmount
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setDebouncedSearch(searchTerm), 400);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [searchTerm]);

  // Reset page when any filter changes
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearch, difficultyFilter, tagFilters, sortField, sortDirection]);

  const setSearchTerm = (v: string) => setSearchTermRaw(v);

  const setDifficultyFilter = (v: Difficulty | "all") => setDifficultyFilterRaw(v);

  const toggleTag = (id: number) => {
    setTagFilters((prev) =>
      prev.includes(id) ? prev.filter((t) => t !== id) : [...prev, id]
    );
  };

  const removeTag = (id: number) => {
    setTagFilters((prev) => prev.filter((t) => t !== id));
  };

  const setSort = (field: SortField) => {
    setSortField((prev) => {
      if (prev === field) {
        setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
        return field;
      }
      setSortDirection("asc");
      return field;
    });
  };

  const setPage = (p: number) => setCurrentPage(p);

  const clearAll = () => {
    setDifficultyFilterRaw("all");
    setTagFilters([]);
    setSortField("none");
    setSortDirection("desc");
    setSearchTermRaw("");
  };

  const hasActiveFilters =
    difficultyFilter !== "all" ||
    tagFilters.length > 0 ||
    sortField !== "none" ||
    searchTerm !== "";

  return {
    searchTerm,
    debouncedSearch,
    difficultyFilter,
    tagFilters,
    sortField,
    sortDirection,
    currentPage,
    setSearchTerm,
    setDifficultyFilter,
    toggleTag,
    removeTag,
    setSort,
    setPage,
    clearAll,
    hasActiveFilters,
  };
}
