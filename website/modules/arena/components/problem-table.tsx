"use client";

import Link from "next/link";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  CheckCircle2,
  Circle,
  CircleDashed,
  SearchX,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  ARENA_SORT_DIRECTION,
  ARENA_SORT_FIELD,
  type ArenaSortDirection,
  type ArenaSortField,
} from "@/constants/arena";
import { APP_ROUTES } from "@/constants/routes";
import { TEXT } from "@/constants/text";
import { formatNumber } from "@/lib/format";
import type { Problem } from "@/types/problem";
import { DifficultyBadge } from "./difficulty-badge";

export type SortField = ArenaSortField;
export type SortDirection = ArenaSortDirection;

interface ProblemTableProps {
  problems: Problem[];
  sortField: SortField;
  sortDirection: SortDirection;
  onSort: (field: SortField) => void;
  hasActiveFilters: boolean;
  onClearFilters: () => void;
}

function acceptanceValue(problem: Problem): string {
  const rate = Number.isFinite(problem.acceptance_rate)
    ? Math.min(100, Math.max(0, problem.acceptance_rate))
    : 0;
  return `${rate.toFixed(1)}%`;
}

function submissionValue(problem: Problem): string {
  return formatNumber(Math.max(0, problem.submission_count));
}

function ProblemStatus({ problem }: { problem: Problem }) {
  if (problem.is_solved === true) {
    return (
      <span className="inline-flex items-center text-status-success">
        <CheckCircle2
          className="size-5 filter-shadow-success"
          aria-hidden="true"
        />
        <span className="sr-only">{TEXT.ARENA.SOLVED_LABEL}</span>
      </span>
    );
  }
  if (problem.is_solved === false) {
    return (
      <span className="inline-flex items-center text-muted-foreground/55">
        <Circle className="size-5" aria-hidden="true" />
        <span className="sr-only">{TEXT.ARENA.UNSOLVED_LABEL}</span>
      </span>
    );
  }
  return (
    <span className="inline-flex items-center text-muted-foreground/55">
      <CircleDashed className="size-5" aria-hidden="true" />
      <span className="sr-only">
        {TEXT.ARENA.SOLVE_STATUS_UNAVAILABLE}
      </span>
    </span>
  );
}

function SortIcon({
  active,
  direction,
}: {
  active: boolean;
  direction: SortDirection;
}) {
  if (!active) {
    return (
      <ArrowUpDown
        className="size-3.5 opacity-45"
        aria-hidden="true"
      />
    );
  }
  return direction === ARENA_SORT_DIRECTION.DESCENDING ? (
    <ArrowDown className="size-3.5 text-primary" aria-hidden="true" />
  ) : (
    <ArrowUp className="size-3.5 text-primary" aria-hidden="true" />
  );
}

function ProblemTags({ problem }: { problem: Problem }) {
  const tags = problem.tags ?? [];
  if (tags.length === 0) {
    return <span className="text-sm text-muted-foreground">{TEXT.COMMON.NOT_AVAILABLE}</span>;
  }
  return (
    <div className="flex min-w-0 flex-wrap gap-1.5">
      {tags.slice(0, 3).map((tag) => (
        <Badge
          key={tag.id}
          variant="secondary"
          className="h-auto max-w-full rounded-full border-0 px-2 py-0.5 text-xs font-medium"
        >
          <span className="truncate">{tag.name}</span>
        </Badge>
      ))}
      {tags.length > 3 ? (
        <Badge
          variant="secondary"
          className="h-auto rounded-full border-0 bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
        >
          +{tags.length - 3}
        </Badge>
      ) : null}
    </div>
  );
}

export function ProblemTable({
  problems,
  sortField,
  sortDirection,
  onSort,
  hasActiveFilters,
  onClearFilters,
}: ProblemTableProps) {
  if (problems.length === 0) {
    return (
      <div className="flex min-h-72 flex-col items-center justify-center rounded-2xl border border-dashed border-border bg-surface/75 px-6 py-12 text-center motion-safe:animate-in motion-safe:fade-in">
        <span className="mb-4 flex size-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
          <SearchX className="size-6" aria-hidden="true" />
        </span>
        <h2 className="text-lg font-semibold text-foreground">
          {hasActiveFilters
            ? TEXT.ARENA.NO_FILTERED_PROBLEMS
            : TEXT.ARENA.NO_PROBLEMS}
        </h2>
        <p className="mt-1 max-w-md text-sm leading-6 text-muted-foreground">
          {hasActiveFilters
            ? TEXT.ARENA.NO_FILTERED_PROBLEMS_DESC
            : TEXT.ARENA.NO_PROBLEMS_DESC}
        </p>
        {hasActiveFilters ? (
          <Button
            type="button"
            variant="outline"
            className="mt-5"
            onClick={onClearFilters}
          >
            {TEXT.ARENA.CLEAR_EMPTY_FILTERS}
          </Button>
        ) : null}
      </div>
    );
  }

  return (
    <>
      <div className="grid gap-3 md:hidden">
        {problems.map((problem) => (
          <Link
            key={problem.id}
            href={APP_ROUTES.ARENA_PROBLEM(problem.id)}
            className="group rounded-2xl border border-border bg-card p-4 shadow-sm outline-none transition-[border-color,box-shadow,background-color] duration-200 hover:border-primary/35 hover:bg-surface-raised hover:shadow-md focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <div className="flex items-start gap-3">
              <span className="mt-0.5 shrink-0">
                <ProblemStatus problem={problem} />
              </span>
              <div className="min-w-0 flex-1">
                <h2 className="line-clamp-2 font-semibold leading-6 text-foreground transition-colors group-hover:text-primary">
                  {problem.title}
                </h2>
                <div className="mt-2">
                  <DifficultyBadge difficulty={problem.difficulty} />
                </div>
              </div>
            </div>

            <div className="mt-3">
              <ProblemTags problem={problem} />
            </div>

            <dl className="mt-4 grid grid-cols-2 gap-2 border-t border-border/60 pt-3 text-sm">
              <div>
                <dt className="text-xs text-muted-foreground">
                  {TEXT.ARENA.SORT_ACCEPTANCE}
                </dt>
                <dd className="mt-0.5 font-mono font-semibold tabular-nums text-foreground">
                  {acceptanceValue(problem)}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">
                  {TEXT.ARENA.SORT_SOLVED}
                </dt>
                <dd className="mt-0.5 font-mono font-semibold tabular-nums text-foreground">
                  {submissionValue(problem)}
                </dd>
              </div>
            </dl>
          </Link>
        ))}
      </div>

      <div className="hidden overflow-hidden rounded-2xl border border-border bg-card shadow-sm motion-safe:animate-in motion-safe:fade-in md:block">
        <Table className="table-fixed">
          <TableCaption className="sr-only">
            {TEXT.ARENA.TABLE_LABEL}
          </TableCaption>
          <TableHeader>
            <TableRow className="bg-surface-sunken/60 hover:bg-surface-sunken/60">
              <TableHead className="w-14 text-center">
                <span className="sr-only">{TEXT.ARENA.STATUS}</span>
              </TableHead>
              <TableHead className="font-semibold">
                {TEXT.ARENA.PROBLEM_TITLE}
              </TableHead>
              <TableHead className="w-32 font-semibold">
                {TEXT.ARENA.DIFFICULTY}
              </TableHead>
              <TableHead className="hidden w-56 font-semibold lg:table-cell">
                {TEXT.ARENA.TAGS_TITLE}
              </TableHead>
              <TableHead className="w-32 p-0 text-center font-semibold">
                <button
                  type="button"
                  className="flex min-h-11 w-full items-center justify-center gap-1.5 px-2 outline-none transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-inset focus-visible:ring-ring/50"
                  aria-pressed={sortField === ARENA_SORT_FIELD.ACCEPTANCE}
                  onClick={() => onSort(ARENA_SORT_FIELD.ACCEPTANCE)}
                >
                  {TEXT.ARENA.SORT_ACCEPTANCE}
                  <SortIcon
                    active={sortField === ARENA_SORT_FIELD.ACCEPTANCE}
                    direction={sortDirection}
                  />
                </button>
              </TableHead>
              <TableHead className="hidden w-32 p-0 text-center font-semibold xl:table-cell">
                <button
                  type="button"
                  className="flex min-h-11 w-full items-center justify-center gap-1.5 px-2 outline-none transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-inset focus-visible:ring-ring/50"
                  aria-pressed={sortField === ARENA_SORT_FIELD.SUBMISSIONS}
                  onClick={() => onSort(ARENA_SORT_FIELD.SUBMISSIONS)}
                >
                  {TEXT.ARENA.SORT_SOLVED}
                  <SortIcon
                    active={sortField === ARENA_SORT_FIELD.SUBMISSIONS}
                    direction={sortDirection}
                  />
                </button>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {problems.map((problem) => (
              <TableRow
                key={problem.id}
                className="group border-border/60 last:border-0"
              >
                <TableCell className="text-center">
                  <ProblemStatus problem={problem} />
                </TableCell>
                <TableCell className="max-w-0">
                  <Link
                    href={APP_ROUTES.ARENA_PROBLEM(problem.id)}
                    className="flex min-h-11 w-full items-center rounded-md font-semibold text-foreground outline-none transition-colors hover:text-primary hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    <span className="truncate" title={problem.title}>
                      {problem.title}
                    </span>
                  </Link>
                </TableCell>
                <TableCell>
                  <DifficultyBadge difficulty={problem.difficulty} />
                </TableCell>
                <TableCell className="hidden lg:table-cell">
                  <ProblemTags problem={problem} />
                </TableCell>
                <TableCell className="text-center font-mono text-sm font-medium tabular-nums text-muted-foreground transition-colors group-hover:text-foreground">
                  {acceptanceValue(problem)}
                </TableCell>
                <TableCell className="hidden text-center font-mono text-sm font-medium tabular-nums text-muted-foreground transition-colors group-hover:text-foreground xl:table-cell">
                  {submissionValue(problem)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  );
}
