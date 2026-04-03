"use client";

/**
 * Client-side Arena component.
 * Fetches problems + tags from real API. Uses real stats (submission_count, acceptance_rate, is_solved).
 */

import { useState, useEffect, useCallback, useRef } from "react";
import { ProblemTable, type SortField, type SortDirection } from "@/modules/arena/components/problem-table";
import { TEXT } from "@/constants/text";
import type { Problem, Difficulty } from "@/types/problem";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";
import { getProblems } from "@/services/problem.service";
import { getAllTags } from "@/services/tag.service";
import { getAllDifficulties } from "@/services/difficulty.service";
import { Search, SlidersHorizontal, X, ArrowDown, ArrowUp, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

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

  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [difficultyFilter, setDifficultyFilter] = useState<Difficulty | "all">("all");
  const [tagFilters, setTagFilters] = useState<number[]>([]);
  const [sortField, setSortField] = useState<SortField>("none");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  // Debounce search input (400ms)
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);
  useEffect(() => {
    if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current);
    debounceTimerRef.current = setTimeout(() => setDebouncedSearch(searchTerm), 400);
  }, [searchTerm]);

  // Load tags + difficulties once (FindAll — no pagination)
  useEffect(() => {
    getAllTags()
      .then(setTags)
      .catch(() => {});
    getAllDifficulties()
      .then(setDifficulties)
      .catch(() => {});
  }, []);

  // Map difficulty slug → id using loaded difficulties
  const getDifficultyId = useCallback((slug: Difficulty): number | null => {
    const levelMap: Record<string, number> = { easy: 1, medium: 2, hard: 3 };
    const diff = difficulties.find(d => d.level === levelMap[slug]);
    return diff?.id ?? null;
  }, [difficulties]);

  // Fetch problems when filters/page change
  const fetchProblems = useCallback(async () => {
    setLoading(true);
    try {
      const filters = [];
      if (debouncedSearch) {
        filters.push({ key: "title", value: debouncedSearch, type: "search" });
      }
      if (difficultyFilter !== "all") {
        const diffId = getDifficultyId(difficultyFilter);
        if (diffId) {
          filters.push({ key: "difficulty_id", value: diffId, type: "filter" });
        }
      }
      if (tagFilters.length > 0) {
        filters.push({ key: "tag_ids", value: tagFilters, type: "filter" });
      }

      const sort = [];
      if (sortField === "acceptance") {
        sort.push({ key: "accepted_count", order: sortDirection === "asc" ? 1 : -1 });
      } else if (sortField === "solved") {
        sort.push({ key: "submission_count", order: sortDirection === "asc" ? 1 : -1 });
      }

      const res = await getProblems({
        pagination: { page: currentPage, page_size: PAGE_SIZE },
        filters: filters.length > 0 ? filters : undefined,
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

  useEffect(() => {
    fetchProblems();
  }, [fetchProblems]);

  // Reset page on filter change
  useEffect(() => {
    setCurrentPage(1);
  }, [debouncedSearch, difficultyFilter, tagFilters, sortField, sortDirection]);

  const removeTagFilter = (id: number) => {
    setTagFilters(prev => prev.filter(t => t !== id));
  };

  const clearAllFilters = () => {
    setDifficultyFilter("all");
    setTagFilters([]);
    setSortField("none");
    setSortDirection("desc");
    setSearchTerm("");
  };

  const hasActiveFilters = difficultyFilter !== "all" || tagFilters.length > 0 || sortField !== "none" || searchTerm !== "";

  return (
    <div className="space-y-6 animate-in fade-in duration-500">

      {/* Page Header Area */}
      <div>
        <h2 className="text-3xl sm:text-4xl font-bold tracking-tight text-transparent bg-clip-text bg-gradient-to-r from-amber-500 to-amber-200 font-playfair italic flex items-center gap-3">
          <div className="flex items-center justify-center p-2 rounded-xl bg-amber-500/10 border border-amber-500/20 shadow-[0_0_15px_rgba(245,158,11,0.2)]">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-swords w-6 h-6 text-amber-500 drop-shadow-[0_0_5px_rgba(245,158,11,0.8)]"><polyline points="14.5 17.5 3 6 3 3 6 3 17.5 14.5"/><line x1="13" x2="19" y1="19" y2="13"/><line x1="16" x2="21" y1="16" y2="21"/><line x1="19" x2="21" y1="21" y2="19"/><polyline points="14.5 6.5 18 3 21 3 21 6 17.5 9.5"/><line x1="5" x2="9" y1="14" y2="18"/><line x1="7" x2="4" y1="17" y2="20"/><line x1="3" x2="5" y1="19" y2="21"/></svg>
          </div>
          {TEXT.ARENA.TITLE}
        </h2>
        <p className="text-muted-foreground mt-2 text-sm font-medium">{TEXT.ARENA.SUBTITLE}</p>
      </div>

      {/* Filter Toolbar */}
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-3 justify-between bg-card/40 backdrop-blur-md border border-border/50 p-2 rounded-2xl shadow-sm">

          {/* Search */}
          <div className="relative flex-1 group">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground group-focus-within:text-primary transition-colors" />
            <Input
              type="text"
              placeholder={TEXT.ARENA.SEARCH_PLACEHOLDER}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full pl-9 pr-4 h-11 bg-background/50 border-border/50 focus-visible:ring-1 focus-visible:ring-primary focus-visible:bg-background rounded-xl transition-all shadow-none text-base"
            />
          </div>

          {/* Filter Sheet */}
          <Sheet>
            <SheetTrigger>
              <Button
                variant="outline"
                className="h-11 px-4 gap-2 rounded-xl shrink-0 border-border/50 bg-background/50 hover:bg-background hover:text-primary transition-colors whitespace-nowrap"
              >
                <SlidersHorizontal className="h-4 w-4" />
                <span className="hidden sm:inline font-medium">Lọc & Sắp Xếp</span>
                {(difficultyFilter !== "all" || tagFilters.length > 0) && (
                  <Badge variant="default" className="ml-1 h-5 w-5 rounded-full p-0 flex items-center justify-center bg-primary text-primary-foreground">
                    {(difficultyFilter !== "all" ? 1 : 0) + tagFilters.length}
                  </Badge>
                )}
              </Button>
            </SheetTrigger>

            <SheetContent side="right" className="w-[85vw] sm:max-w-md border-l border-border/50 glass-card p-0 flex flex-col gap-0 backdrop-blur-2xl bg-background/80">
              <SheetHeader className="p-6 border-b border-border/40">
                <SheetTitle className="text-xl font-bold flex items-center gap-2">
                  <SlidersHorizontal className="h-5 w-5 text-primary" />
                  {TEXT.ARENA.FILTER_TITLE}
                </SheetTitle>
              </SheetHeader>

              <div className="flex-1 overflow-y-auto p-6 space-y-8 no-scrollbar">

                {/* SORTING */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.SORT_TITLE}</h3>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      onClick={() => setSortField(sortField === "acceptance" ? "none" : "acceptance")}
                      className={`flex items-center justify-center p-3 rounded-xl border text-sm font-medium transition-all ${sortField === "acceptance" ? "bg-primary/10 border-primary text-primary shadow-[0_0_10px_rgba(245,158,11,0.1)]" : "bg-card/50 border-border/50 text-muted-foreground hover:border-primary/50"}`}
                    >
                      {TEXT.ARENA.SORT_ACCEPTANCE}
                    </button>
                    <button
                      onClick={() => setSortField(sortField === "solved" ? "none" : "solved")}
                      className={`flex items-center justify-center p-3 rounded-xl border text-sm font-medium transition-all ${sortField === "solved" ? "bg-primary/10 border-primary text-primary shadow-[0_0_10px_rgba(245,158,11,0.1)]" : "bg-card/50 border-border/50 text-muted-foreground hover:border-primary/50"}`}
                    >
                      {TEXT.ARENA.SORT_SOLVED}
                    </button>
                  </div>

                  <div className={`flex bg-muted/30 p-1 rounded-xl border border-border/30 mt-2 transition-opacity ${sortField === 'none' ? 'opacity-50 pointer-events-none' : 'opacity-100'}`}>
                    <button
                      onClick={() => setSortDirection("desc")}
                      className={`flex-1 flex items-center justify-center gap-2 p-2 rounded-lg text-sm font-medium transition-all ${sortDirection === "desc" ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                    >
                      <ArrowDown className="h-4 w-4" /> {TEXT.ARENA.DESC}
                    </button>
                    <button
                      onClick={() => setSortDirection("asc")}
                      className={`flex-1 flex items-center justify-center gap-2 p-2 rounded-lg text-sm font-medium transition-all ${sortDirection === "asc" ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                    >
                      <ArrowUp className="h-4 w-4" /> {TEXT.ARENA.ASC}
                    </button>
                  </div>
                </div>

                {/* DIFFICULTY */}
                <div className="space-y-4">
                  <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.DIFFICULTY_TITLE}</h3>
                  <div className="grid grid-cols-3 gap-2">
                    {(["easy", "medium", "hard"] as Difficulty[]).map(diff => {
                      const isActive = difficultyFilter === diff;
                      const colors = {
                        easy: isActive ? "bg-emerald-500/10 border-emerald-500 text-emerald-500" : "hover:border-emerald-500/50 text-muted-foreground",
                        medium: isActive ? "bg-amber-500/10 border-amber-500 text-amber-500" : "hover:border-amber-500/50 text-muted-foreground",
                        hard: isActive ? "bg-rose-500/10 border-rose-500 text-rose-500" : "hover:border-rose-500/50 text-muted-foreground",
                      };
                      const labels = { easy: TEXT.ARENA.EASY, medium: TEXT.ARENA.MEDIUM, hard: TEXT.ARENA.HARD };

                      return (
                        <button
                          key={diff}
                          onClick={() => setDifficultyFilter(isActive ? "all" : diff)}
                          className={`flex items-center justify-center p-3 rounded-xl border border-border/50 bg-card/50 text-sm font-medium transition-all ${colors[diff]}`}
                        >
                          {labels[diff]}
                        </button>
                      );
                    })}
                  </div>
                </div>

                {/* TAGS (from API) */}
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.TAGS_TITLE}</h3>
                    {tagFilters.length > 0 && (
                      <button onClick={() => setTagFilters([])} className="text-xs text-primary hover:underline">{TEXT.ARENA.CLEAR_ALL}</button>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {tags.map(tag => {
                      const isActive = tagFilters.includes(tag.id);
                      return (
                        <button
                          key={tag.id}
                          onClick={() => {
                            setTagFilters(prev => isActive ? prev.filter(t => t !== tag.id) : [...prev, tag.id]);
                          }}
                          className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-sm font-medium transition-all ${isActive ? "bg-violet-500/20 border-violet-500/50 text-violet-400" : "bg-card/40 border-border/40 text-muted-foreground hover:border-border hover:bg-card/80"}`}
                        >
                          {tag.name}
                        </button>
                      );
                    })}
                  </div>
                </div>

              </div>

              <div className="p-6 border-t border-border/40 bg-muted/10">
                <Button onClick={clearAllFilters} variant="outline" className="w-full rounded-xl h-11 border-border/50 hover:bg-background">
                  {TEXT.ARENA.CLEAR_FILTER}
                </Button>
              </div>
            </SheetContent>
          </Sheet>
        </div>

        {/* Active Filters Display */}
        {hasActiveFilters && (
          <div className="flex items-center flex-wrap gap-2 px-2 animate-in slide-in-from-top-2 opacity-100 duration-300">
            <span className="text-xs text-muted-foreground font-medium mr-1">{TEXT.ARENA.ACTIVE_FILTERS}</span>

            {difficultyFilter !== "all" && (
              <Badge variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-border/50 bg-background/50 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors" onClick={() => setDifficultyFilter("all")}>
                {TEXT.ARENA.DIFFICULTY}: {difficultyFilter === 'easy' ? TEXT.ARENA.EASY : difficultyFilter === 'medium' ? TEXT.ARENA.MEDIUM : TEXT.ARENA.HARD}
                <X className="h-3 w-3 opacity-70" />
              </Badge>
            )}

            {tagFilters.map(id => {
              const tag = tags.find(t => t.id === id);
              if (!tag) return null;
              return (
                <Badge key={id} variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-border/50 bg-background/50 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors" onClick={() => removeTagFilter(id)}>
                  {tag.name}
                  <X className="h-3 w-3 opacity-70" />
                </Badge>
              );
            })}

            {sortField !== "none" && (
              <Badge variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-primary/30 bg-primary/5 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors text-primary" onClick={() => setSortField("none")}>
                Sắp xếp: {sortField === 'acceptance' ? TEXT.ARENA.SORT_ACCEPTANCE : TEXT.ARENA.SORT_SOLVED} {sortDirection === 'desc' ? '↓' : '↑'}
                <X className="h-3 w-3 opacity-70" />
              </Badge>
            )}

            <button onClick={clearAllFilters} className="text-xs text-muted-foreground hover:text-primary transition-colors ml-2 font-medium">
              {TEXT.ARENA.CLEAR_ALL}
            </button>
          </div>
        )}
      </div>

      {/* Problem Table */}
      <div className="space-y-4">
        {loading ? (
          <div className="flex items-center justify-center p-12">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        ) : (
          <ProblemTable
            problems={problems}
            sortField={sortField}
            sortDirection={sortDirection}
            onSort={(field) => {
              if (sortField === field) {
                setSortDirection(prev => prev === 'asc' ? 'desc' : 'asc');
              } else {
                setSortField(field);
                setSortDirection('asc');
              }
            }}
          />
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 px-2 pt-2 pb-6">
            <p className="text-sm text-muted-foreground order-2 sm:order-1">
              Hiển thị <span className="font-medium text-foreground">{currentPage}</span> / <span className="font-medium text-foreground">{totalPages}</span> {TEXT.COMMON.PAGE.toLowerCase()}
            </p>
            <div className="flex items-center gap-2 order-1 sm:order-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="rounded-xl h-9 px-3"
              >
                {TEXT.COMMON.PREVIOUS}
              </Button>
              <div className="flex items-center gap-1.5">
                {getVisiblePages(currentPage, totalPages).map((page, index) => {
                  if (page === '...') {
                    return (
                      <span key={`ellipsis-${index}`} className="w-9 h-9 flex items-center justify-center text-muted-foreground font-medium">
                        ...
                      </span>
                    );
                  }
                  const pageNum = page as number;
                  const isActive = currentPage === pageNum;
                  return (
                    <Button
                      key={pageNum}
                      variant={isActive ? "default" : "ghost"}
                      size="sm"
                      onClick={() => setCurrentPage(pageNum)}
                      className={`w-9 h-9 p-0 rounded-lg text-sm font-medium transition-all duration-300 ${isActive ? 'shadow-[0_0_15px_rgba(245,158,11,0.5)] scale-110 bg-primary text-primary-foreground border border-amber-400/50' : 'hover:bg-muted/50 hover:scale-105 hover:text-primary'}`}
                    >
                      {pageNum}
                    </Button>
                  );
                })}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                disabled={currentPage === totalPages}
                className="rounded-xl h-9 px-3"
              >
                {TEXT.COMMON.NEXT}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
