"use client";

/**
 * Client-side Arena page.
 * Fetches problems using filter/sort/page state from useArenaFilters.
 * Filter UI delegated to ArenaFilterBar.
 */

import { useState, useEffect, useCallback } from "react";
import { ProblemTable } from "@/modules/arena/components/problem-table";
import { ArenaFilterBar } from "@/modules/arena/components/arena-filter-bar";
import { LoadingSpinner } from "@/components/loading-spinner";
import { useArenaFilters } from "@/modules/arena/hooks/use-arena-filters";
import { TEXT } from "@/constants/text";
import type { Problem } from "@/types/problem";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";
import { getProblems } from "@/services/problem.service";
import { getAllTags } from "@/services/tag.service";
import { getAllDifficulties } from "@/services/difficulty.service";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

const getVisiblePages = (current: number, total: number, siblings = 1) => {
  const pages: (number | string)[] = [];
  for (let i = 1; i <= total; i++) {
    if (i === 1 || i === total || (i >= current - siblings && i <= current + siblings)) {
      pages.push(i);
    } else if (pages[pages.length - 1] !== "...") {
      pages.push("...");
    }
  }
  return pages;
};

export function ArenaClient() {
  const [problems, setProblems] = useState<Problem[]>([]);
  const [tags, setTags] = useState<Tag[]>([]);
  const [difficulties, setDifficulties] = useState<DifficultyResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [totalPages, setTotalPages] = useState(1);

  const filters = useArenaFilters();
  const { debouncedSearch, difficultyFilter, tagFilters, sortField, sortDirection, currentPage, setPage } = filters;

  // Load reference data once
  useEffect(() => {
    getAllTags().then(setTags).catch(() => {});
    getAllDifficulties().then(setDifficulties).catch(() => {});
  }, []);

  // Map difficulty slug → id using loaded difficulties
  const getDifficultyId = useCallback((slug: string): number | null => {
    const levelMap: Record<string, number> = { easy: 1, medium: 2, hard: 3 };
    const diff = difficulties.find((d) => d.level === levelMap[slug]);
    return diff?.id ?? null;
  }, [difficulties]);

  // Fetch problems when filters/page change
  const fetchProblems = useCallback(async () => {
    setLoading(true);
    try {
      const filterList = [];
      if (debouncedSearch) filterList.push({ key: "title", value: debouncedSearch, type: "search" });
      if (difficultyFilter !== "all") {
        const diffId = getDifficultyId(difficultyFilter);
        if (diffId) filterList.push({ key: "difficulty_id", value: diffId, type: "filter" });
      }
      if (tagFilters.length > 0) filterList.push({ key: "tag_ids", value: tagFilters, type: "filter" });

      const sort = [];
      if (sortField === "acceptance") sort.push({ key: "accepted_count", order: sortDirection === "asc" ? 1 : -1 });
      else if (sortField === "solved") sort.push({ key: "submission_count", order: sortDirection === "asc" ? 1 : -1 });

      const res = await getProblems({
        pagination: { page: currentPage, page_size: PAGE_SIZE },
        filters: filterList.length > 0 ? filterList : undefined,
        sort: sort.length > 0 ? sort : undefined,
      });
      setProblems(res.records ?? []);
      setTotalPages(res.pagination.total_pages);
    } catch {
      setProblems([]);
    } finally {
      setLoading(false);
    }
  }, [debouncedSearch, difficultyFilter, tagFilters, sortField, sortDirection, currentPage, getDifficultyId]);

  useEffect(() => { fetchProblems(); }, [fetchProblems]);

  return (
    <div className="space-y-6 animate-in fade-in duration-500">
      {/* Header */}
      <div>
        <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-amber-500 to-amber-200 font-playfair italic flex items-center gap-3">
          <div className="flex items-center justify-center p-2 rounded-xl bg-amber-500/10 border border-amber-500/20 shadow-[0_0_15px_rgba(245,158,11,0.2)]">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-swords w-6 h-6 text-amber-500 drop-shadow-[0_0_5px_rgba(245,158,11,0.8)]"><polyline points="14.5 17.5 3 6 3 3 6 3 17.5 14.5"/><line x1="13" x2="19" y1="19" y2="13"/><line x1="16" x2="21" y1="16" y2="21"/><line x1="19" x2="21" y1="19" y2="21"/><polyline points="14.5 6.5 18 3 21 3 21 6 17.5 9.5"/><line x1="5" x2="9" y1="14" y2="18"/><line x1="7" x2="4" y1="17" y2="20"/><line x1="3" x2="5" y1="19" y2="21"/></svg>
          </div>
          {TEXT.ARENA.TITLE}
        </h2>
        <p className="text-muted-foreground mt-2 text-sm font-medium">{TEXT.ARENA.SUBTITLE}</p>
      </div>

      {/* Filter bar */}
      <ArenaFilterBar filters={filters} tags={tags} difficulties={difficulties} />

      {/* Table + pagination */}
      <div className="space-y-4">
        {loading ? (
          <LoadingSpinner className="py-12" />
        ) : (
          <ProblemTable
            problems={problems}
            sortField={filters.sortField}
            sortDirection={filters.sortDirection}
            onSort={filters.setSort}
          />
        )}

        {totalPages > 1 && (
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 px-2 pt-2 pb-6">
            <p className="text-sm text-muted-foreground order-2 sm:order-1">
              {TEXT.COMMON.PAGE} <span className="font-medium text-foreground">{currentPage}</span> / <span className="font-medium text-foreground">{totalPages}</span>
            </p>
            <div className="flex items-center gap-2 order-1 sm:order-2">
              <Button variant="outline" size="sm" onClick={() => setPage(Math.max(1, currentPage - 1))} disabled={currentPage === 1} className="rounded-xl h-9 px-3">
                {TEXT.COMMON.PREVIOUS}
              </Button>
              <div className="flex items-center gap-1.5">
                {getVisiblePages(currentPage, totalPages).map((page, index) => {
                  if (page === "...") return <span key={`e-${index}`} className="w-9 h-9 flex items-center justify-center text-muted-foreground font-medium">...</span>;
                  const p = page as number;
                  return (
                    <Button key={p} variant={currentPage === p ? "default" : "ghost"} size="sm" onClick={() => setPage(p)}
                      className={`w-9 h-9 p-0 rounded-lg text-sm font-medium transition-all duration-300 ${currentPage === p ? "shadow-[0_0_15px_rgba(245,158,11,0.5)] scale-110 bg-primary text-primary-foreground border border-amber-400/50" : "hover:bg-muted/50 hover:scale-105 hover:text-primary"}`}>
                      {p}
                    </Button>
                  );
                })}
              </div>
              <Button variant="outline" size="sm" onClick={() => setPage(Math.min(totalPages, currentPage + 1))} disabled={currentPage === totalPages} className="rounded-xl h-9 px-3">
                {TEXT.COMMON.NEXT}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
