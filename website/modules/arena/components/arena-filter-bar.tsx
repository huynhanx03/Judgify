"use client";

/**
 * Filter toolbar for the Arena page.
 * Renders: search input, filter/sort Sheet, active filter badges.
 * Zero fetch logic — all state comes from useArenaFilters.
 */

import { Search, SlidersHorizontal, X, ArrowDown, ArrowUp } from "lucide-react";
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
import { TEXT } from "@/constants/text";
import { getDifficultyStyle } from "@/constants/styles";
import type { UseArenaFiltersReturn, Difficulty } from "@/modules/arena/hooks/use-arena-filters";
import type { Tag } from "@/types/tag";
import type { DifficultyResponse } from "@/types/difficulty";

interface ArenaFilterBarProps {
  filters: UseArenaFiltersReturn;
  tags: Tag[];
  difficulties: DifficultyResponse[];
}

export function ArenaFilterBar({ filters, tags, difficulties }: ArenaFilterBarProps) {
  const {
    searchTerm, setSearchTerm,
    difficultyFilter, setDifficultyFilter,
    tagFilters, toggleTag, removeTag,
    sortField, setSort,
    sortDirection,
    clearAll, hasActiveFilters,
  } = filters;

  const activeFilterCount = (difficultyFilter !== "all" ? 1 : 0) + tagFilters.length;

  return (
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
              <span className="hidden sm:inline font-medium">{TEXT.ARENA.FILTER_TITLE}</span>
              {activeFilterCount > 0 && (
                <Badge variant="default" className="ml-1 h-5 w-5 rounded-full p-0 flex items-center justify-center bg-primary text-primary-foreground">
                  {activeFilterCount}
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

              {/* Sort */}
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.SORT_TITLE}</h3>
                <div className="grid grid-cols-2 gap-2">
                  {(["acceptance", "solved"] as const).map((field) => (
                    <button
                      key={field}
                      onClick={() => setSort(field)}
                      className={`flex items-center justify-center p-3 rounded-xl border text-sm font-medium transition-all ${
                        sortField === field
                          ? "bg-primary/10 border-primary text-primary shadow-[0_0_10px_rgba(245,158,11,0.1)]"
                          : "bg-card/50 border-border/50 text-muted-foreground hover:border-primary/50"
                      }`}
                    >
                      {field === "acceptance" ? TEXT.ARENA.SORT_ACCEPTANCE : TEXT.ARENA.SORT_SOLVED}
                    </button>
                  ))}
                </div>
                <div className={`flex bg-muted/30 p-1 rounded-xl border border-border/30 mt-2 transition-opacity ${sortField === "none" ? "opacity-50 pointer-events-none" : "opacity-100"}`}>
                  {(["desc", "asc"] as const).map((dir) => (
                    <button
                      key={dir}
                      onClick={() => { if (sortField !== "none") setSort(sortField); }}
                      className={`flex-1 flex items-center justify-center gap-2 p-2 rounded-lg text-sm font-medium transition-all ${sortDirection === dir ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                    >
                      {dir === "desc" ? <ArrowDown className="h-4 w-4" /> : <ArrowUp className="h-4 w-4" />}
                      {dir === "desc" ? TEXT.ARENA.DESC : TEXT.ARENA.ASC}
                    </button>
                  ))}
                </div>
              </div>

              {/* Difficulty */}
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.DIFFICULTY_TITLE}</h3>
                <div className="grid grid-cols-3 gap-2">
                  {difficulties.map((diff) => {
                    const isActive = difficultyFilter === (diff.level === 1 ? "easy" : diff.level === 2 ? "medium" : "hard");
                    const slug: Difficulty = diff.level === 1 ? "easy" : diff.level === 2 ? "medium" : "hard";
                    const style = getDifficultyStyle(diff.level);
                    return (
                      <button
                        key={diff.id}
                        onClick={() => setDifficultyFilter(isActive ? "all" : slug)}
                        className={`flex items-center justify-center p-3 rounded-xl border border-border/50 bg-card/50 text-sm font-medium transition-all ${isActive ? style.active : style.hover}`}
                      >
                        {diff.name}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Tags */}
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{TEXT.ARENA.TAGS_TITLE}</h3>
                  {tagFilters.length > 0 && (
                    <button onClick={() => tagFilters.forEach(removeTag)} className="text-xs text-primary hover:underline">{TEXT.ARENA.CLEAR_ALL}</button>
                  )}
                </div>
                <div className="flex flex-wrap gap-2">
                  {tags.map((tag) => {
                    const isActive = tagFilters.includes(tag.id);
                    return (
                      <button
                        key={tag.id}
                        onClick={() => toggleTag(tag.id)}
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
              <Button onClick={clearAll} variant="outline" className="w-full rounded-xl h-11 border-border/50 hover:bg-background">
                {TEXT.ARENA.CLEAR_FILTER}
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      </div>

      {/* Active filter badges */}
      {hasActiveFilters && (
        <div className="flex items-center flex-wrap gap-2 px-2 animate-in slide-in-from-top-2 opacity-100 duration-300">
          <span className="text-xs text-muted-foreground font-medium mr-1">{TEXT.ARENA.ACTIVE_FILTERS}</span>

          {difficultyFilter !== "all" && (
            <Badge variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-border/50 bg-background/50 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors"
              onClick={() => setDifficultyFilter("all")}>
              {TEXT.ARENA.DIFFICULTY}: {difficultyFilter === "easy" ? TEXT.ARENA.EASY : difficultyFilter === "medium" ? TEXT.ARENA.MEDIUM : TEXT.ARENA.HARD}
              <X className="h-3 w-3 opacity-70" />
            </Badge>
          )}

          {tagFilters.map((id) => {
            const tag = tags.find((t) => t.id === id);
            if (!tag) return null;
            return (
              <Badge key={id} variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-border/50 bg-background/50 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors"
                onClick={() => removeTag(id)}>
                {tag.name}
                <X className="h-3 w-3 opacity-70" />
              </Badge>
            );
          })}

          {sortField !== "none" && (
            <Badge variant="outline" className="h-7 px-2.5 gap-1.5 rounded-full border-primary/30 bg-primary/5 backdrop-blur-sm text-xs font-medium cursor-pointer hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-colors text-primary"
              onClick={() => setSort(sortField)}>
              {TEXT.ARENA.SORT_TITLE}: {sortField === "acceptance" ? TEXT.ARENA.SORT_ACCEPTANCE : TEXT.ARENA.SORT_SOLVED} {sortDirection === "desc" ? "↓" : "↑"}
              <X className="h-3 w-3 opacity-70" />
            </Badge>
          )}

          <button onClick={clearAll} className="text-xs text-muted-foreground hover:text-primary transition-colors ml-2 font-medium">
            {TEXT.ARENA.CLEAR_ALL}
          </button>
        </div>
      )}
    </div>
  );
}
