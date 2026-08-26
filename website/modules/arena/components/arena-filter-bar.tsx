"use client";

import {
  ArrowDown,
  ArrowUp,
  Loader2,
  Search,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  ARENA_POLICY,
  ARENA_SORT_DIRECTION,
  ARENA_SORT_FIELD,
} from "@/constants/arena";
import { getDifficultyStyle } from "@/constants/styles";
import { TEXT } from "@/constants/text";
import type { UseArenaFiltersReturn } from "@/modules/arena/hooks/use-arena-filters";
import type { DifficultyResponse } from "@/types/difficulty";
import type { Tag } from "@/types/tag";

interface ArenaFilterBarProps {
  filters: UseArenaFiltersReturn;
  tags: Tag[];
  difficulties: DifficultyResponse[];
  referenceDataLoading: boolean;
}

export function ArenaFilterBar({
  filters,
  tags,
  difficulties,
  referenceDataLoading,
}: ArenaFilterBarProps) {
  const {
    searchTerm,
    setSearchTerm,
    difficultyFilter,
    setDifficultyFilter,
    tagFilters,
    tagLimitReached,
    toggleTag,
    removeTag,
    clearTags,
    sortField,
    setSort,
    setSortDirection,
    sortDirection,
    clearAll,
    hasActiveFilters,
  } = filters;
  const activeFilterCount =
    (difficultyFilter !== null ? 1 : 0) +
    tagFilters.length +
    (sortField !== ARENA_SORT_FIELD.NONE ? 1 : 0);

  return (
    <section className="flex flex-col gap-3" aria-label={TEXT.ARENA.FILTER_TITLE}>
      <div className="flex items-center justify-between gap-2 rounded-2xl border border-border bg-card p-2 shadow-sm sm:gap-3">
        <div className="group relative min-w-0 flex-1">
          <label htmlFor="arena-problem-search" className="sr-only">
            {TEXT.ARENA.SEARCH_LABEL}
          </label>
          <Search
            className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground transition-colors group-focus-within:text-primary"
            aria-hidden="true"
          />
          <Input
            id="arena-problem-search"
            type="search"
            placeholder={TEXT.ARENA.SEARCH_PLACEHOLDER}
            value={searchTerm}
            maxLength={ARENA_POLICY.MAXIMUM_SEARCH_CHARACTERS}
            onChange={(event) => setSearchTerm(event.target.value)}
            className="h-11 w-full rounded-xl border-border bg-surface-sunken/50 pl-9 pr-3 text-base shadow-none transition-colors focus-visible:bg-surface"
          />
        </div>

        <Sheet>
          <SheetTrigger
            render={
              <Button
                type="button"
                variant="outline"
                aria-label={TEXT.ARENA.FILTER_TITLE}
                aria-busy={referenceDataLoading}
                className="h-11 shrink-0 gap-2 rounded-xl px-3 sm:px-4"
              />
            }
          >
            {referenceDataLoading ? (
              <Loader2
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : (
              <SlidersHorizontal className="size-4" aria-hidden="true" />
            )}
            <span className="hidden font-medium sm:inline">
              {TEXT.ARENA.FILTER_TITLE}
            </span>
            {activeFilterCount > 0 ? (
              <Badge className="ml-0.5 flex size-5 items-center justify-center rounded-full p-0 text-[11px]">
                {activeFilterCount}
              </Badge>
            ) : null}
          </SheetTrigger>

          <SheetContent
            side="right"
            className="flex w-[min(92vw,28rem)] flex-col gap-0 border-l border-border bg-surface-raised/95 p-0 backdrop-blur-xl"
          >
            <SheetHeader className="border-b border-border p-5 sm:p-6">
              <SheetTitle className="flex items-center gap-2 text-xl font-bold">
                <SlidersHorizontal
                  className="size-5 text-primary"
                  aria-hidden="true"
                />
                {TEXT.ARENA.FILTER_TITLE}
              </SheetTitle>
              <SheetDescription>
                {TEXT.ARENA.FILTER_DESCRIPTION}
              </SheetDescription>
            </SheetHeader>

            <div className="flex-1 space-y-8 overflow-y-auto p-5 sm:p-6">
              <fieldset className="space-y-4">
                <legend className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                  {TEXT.ARENA.SORT_TITLE}
                </legend>
                <div className="grid grid-cols-2 gap-2">
                  {[
                    {
                      field: ARENA_SORT_FIELD.ACCEPTANCE,
                      label: TEXT.ARENA.SORT_ACCEPTANCE,
                    },
                    {
                      field: ARENA_SORT_FIELD.SUBMISSIONS,
                      label: TEXT.ARENA.SORT_SOLVED,
                    },
                  ].map(({ field, label }) => {
                    const active = sortField === field;
                    return (
                      <button
                        key={field}
                        type="button"
                        onClick={() => setSort(field)}
                        aria-pressed={active}
                        className={`flex min-h-11 items-center justify-center rounded-xl border p-3 text-sm font-medium transition-colors ${
                          active
                            ? "border-primary bg-primary/10 text-primary"
                            : "border-border bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground"
                        }`}
                      >
                        {label}
                      </button>
                    );
                  })}
                </div>
                <div
                  className={`flex rounded-xl border border-border bg-surface-sunken p-1 transition-opacity ${
                    sortField === ARENA_SORT_FIELD.NONE
                      ? "opacity-55"
                      : "opacity-100"
                  }`}
                >
                  {[
                    {
                      direction: ARENA_SORT_DIRECTION.DESCENDING,
                      label: TEXT.ARENA.DESC,
                      Icon: ArrowDown,
                    },
                    {
                      direction: ARENA_SORT_DIRECTION.ASCENDING,
                      label: TEXT.ARENA.ASC,
                      Icon: ArrowUp,
                    },
                  ].map(({ direction, label, Icon }) => (
                    <button
                      key={direction}
                      type="button"
                      disabled={sortField === ARENA_SORT_FIELD.NONE}
                      onClick={() => setSortDirection(direction)}
                      aria-pressed={sortDirection === direction}
                      className={`flex min-h-11 flex-1 items-center justify-center gap-2 rounded-lg p-2 text-sm font-medium transition-colors disabled:cursor-not-allowed ${
                        sortDirection === direction &&
                        sortField !== ARENA_SORT_FIELD.NONE
                          ? "bg-surface-raised text-foreground shadow-sm"
                          : "text-muted-foreground hover:text-foreground"
                      }`}
                    >
                      <Icon className="size-4" aria-hidden="true" />
                      {label}
                    </button>
                  ))}
                </div>
              </fieldset>

              <fieldset className="space-y-4" aria-busy={referenceDataLoading}>
                <legend className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                  {TEXT.ARENA.DIFFICULTY_TITLE}
                </legend>
                {referenceDataLoading && difficulties.length === 0 ? (
                  <p
                    className="flex min-h-11 items-center gap-2 text-sm text-muted-foreground"
                    role="status"
                  >
                    <Loader2
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                    {TEXT.ARENA.FILTERS_LOADING}
                  </p>
                ) : difficulties.length === 0 ? (
                  <p className="text-sm leading-6 text-muted-foreground">
                    {TEXT.ARENA.NO_DIFFICULTIES}
                  </p>
                ) : (
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                    {difficulties.map((difficulty) => {
                      const active = difficultyFilter === difficulty.id;
                      const style = getDifficultyStyle(difficulty.level);
                      return (
                        <button
                          key={difficulty.id}
                          type="button"
                          onClick={() =>
                            setDifficultyFilter(
                              active ? null : difficulty.id,
                            )
                          }
                          aria-pressed={active}
                          className={`flex min-h-11 items-center justify-center rounded-xl border p-3 text-sm font-medium transition-colors ${
                            active ? style.active : style.hover
                          }`}
                        >
                          {difficulty.name}
                        </button>
                      );
                    })}
                  </div>
                )}
              </fieldset>

              <div
                className="space-y-4"
                aria-busy={referenceDataLoading}
                role="group"
                aria-labelledby="arena-tags-filter-label"
              >
                <div className="flex items-center justify-between gap-3">
                  <h3
                    id="arena-tags-filter-label"
                    className="text-sm font-semibold uppercase tracking-wider text-muted-foreground"
                  >
                    {TEXT.ARENA.TAGS_TITLE}
                  </h3>
                  {tagFilters.length > 0 ? (
                    <button
                      type="button"
                      onClick={clearTags}
                      className="min-h-11 rounded-lg px-2 text-xs font-medium text-primary outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                    >
                      {TEXT.ARENA.CLEAR_ALL}
                    </button>
                  ) : null}
                </div>
                {referenceDataLoading && tags.length === 0 ? (
                  <p
                    className="flex min-h-11 items-center gap-2 text-sm text-muted-foreground"
                    role="status"
                  >
                    <Loader2
                      className="size-4 animate-spin motion-reduce:animate-none"
                      aria-hidden="true"
                    />
                    {TEXT.ARENA.FILTERS_LOADING}
                  </p>
                ) : tags.length === 0 ? (
                  <p className="text-sm leading-6 text-muted-foreground">
                    {TEXT.ARENA.NO_TAGS}
                  </p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {tags.map((tag) => {
                      const active = tagFilters.includes(tag.id);
                      const disabled = tagLimitReached && !active;
                      return (
                        <button
                          key={tag.id}
                          type="button"
                          disabled={disabled}
                          onClick={() => toggleTag(tag.id)}
                          aria-pressed={active}
                          aria-describedby={
                            disabled ? "arena-tag-limit" : undefined
                          }
                          className={`flex min-h-11 items-center rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-45 ${
                            active
                              ? "border-primary/50 bg-primary/10 text-primary"
                              : "border-border bg-card text-muted-foreground hover:border-primary/35 hover:text-foreground"
                          }`}
                        >
                          {tag.name}
                        </button>
                      );
                    })}
                  </div>
                )}
                {tagLimitReached ? (
                  <p
                    id="arena-tag-limit"
                    className="text-xs leading-5 text-status-warning"
                  >
                    {TEXT.ARENA.TAG_LIMIT_REACHED(
                      ARENA_POLICY.MAXIMUM_TAG_FILTERS,
                    )}
                  </p>
                ) : null}
              </div>
            </div>

            <div className="border-t border-border bg-surface-sunken/50 p-5 sm:p-6">
              <Button
                type="button"
                onClick={clearAll}
                variant="outline"
                disabled={!hasActiveFilters}
                className="w-full rounded-xl"
              >
                {TEXT.ARENA.CLEAR_FILTER}
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      </div>

      {hasActiveFilters ? (
        <div className="flex flex-wrap items-center gap-2 px-1 motion-safe:animate-in motion-safe:slide-in-from-top-1">
          <span className="mr-1 text-xs font-medium text-muted-foreground">
            {TEXT.ARENA.ACTIVE_FILTERS}
          </span>

          {difficultyFilter !== null ? (
            <button
              type="button"
              className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-border bg-surface px-3 text-xs font-medium outline-none transition-colors hover:border-destructive/30 hover:bg-destructive/10 hover:text-destructive focus-visible:ring-3 focus-visible:ring-ring/50"
              onClick={() => setDifficultyFilter(null)}
            >
              {TEXT.ARENA.DIFFICULTY}:{" "}
              {difficulties.find(
                (difficulty) => difficulty.id === difficultyFilter,
              )?.name ?? TEXT.COMMON.UNKNOWN}
              <X className="size-3" aria-hidden="true" />
            </button>
          ) : null}

          {tagFilters.map((id) => {
            const tag = tags.find((candidate) => candidate.id === id);
            if (!tag) return null;
            return (
              <button
                key={id}
                type="button"
                className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-border bg-surface px-3 text-xs font-medium outline-none transition-colors hover:border-destructive/30 hover:bg-destructive/10 hover:text-destructive focus-visible:ring-3 focus-visible:ring-ring/50"
                onClick={() => removeTag(id)}
              >
                {tag.name}
                <X className="size-3" aria-hidden="true" />
              </button>
            );
          })}

          {sortField !== ARENA_SORT_FIELD.NONE ? (
            <button
              type="button"
              className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-primary/30 bg-primary/5 px-3 text-xs font-medium text-primary outline-none transition-colors hover:border-destructive/30 hover:bg-destructive/10 hover:text-destructive focus-visible:ring-3 focus-visible:ring-ring/50"
              onClick={() => setSort(ARENA_SORT_FIELD.NONE)}
            >
              {TEXT.ARENA.SORT_TITLE}:{" "}
              {sortField === ARENA_SORT_FIELD.ACCEPTANCE
                ? TEXT.ARENA.SORT_ACCEPTANCE
                : TEXT.ARENA.SORT_SOLVED}
              {sortDirection === ARENA_SORT_DIRECTION.DESCENDING ? (
                <ArrowDown className="size-3" aria-hidden="true" />
              ) : (
                <ArrowUp className="size-3" aria-hidden="true" />
              )}
              <X className="size-3" aria-hidden="true" />
            </button>
          ) : null}

          <button
            type="button"
            onClick={clearAll}
            className="min-h-11 rounded-lg px-2 text-xs font-medium text-muted-foreground outline-none transition-colors hover:text-primary focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {TEXT.ARENA.CLEAR_ALL}
          </button>
        </div>
      ) : null}
    </section>
  );
}
