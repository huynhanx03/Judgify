"use client";

/**
 * Reusable pagination component with page number buttons.
 * Shows: prev, page numbers (with ellipsis), next, and total items info.
 */

import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import type { PaginationMeta } from "@/types/api";
import { cn } from "@/lib/utils";
import { text } from "@/i18n/text";

interface PaginationControlsProps {
  pagination: PaginationMeta;
  onPageChange: (page: number) => void;
}

/** Compute visible page numbers with ellipsis markers (represented as 0). */
function getPageNumbers(current: number, total: number): number[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);

  const pages: number[] = [1];
  const left = Math.max(2, current - 1);
  const right = Math.min(total - 1, current + 1);

  if (left > 2) pages.push(0); // ellipsis
  for (let i = left; i <= right; i++) pages.push(i);
  if (right < total - 1) pages.push(0); // ellipsis
  pages.push(total);

  return pages;
}

export function PaginationControls({ pagination, onPageChange }: PaginationControlsProps) {
  const { current_page, total_pages, total_items, has_prev, has_next } = pagination;
  if (total_pages <= 1) return null;

  const pages = getPageNumbers(current_page, total_pages);

  return (
    <nav
      className="flex flex-wrap items-center justify-between gap-3 text-sm"
      aria-label={text("COMMON.PAGINATION")}
    >
      <span className="text-muted-foreground">
        {text("COMMON.ITEM_COUNT", { count: total_items })}
      </span>
      <div className="flex w-full items-center justify-between gap-3 sm:hidden">
        <Button
          variant="outline"
          size="icon"
          type="button"
          className="size-11 cursor-pointer"
          disabled={!has_prev}
          onClick={() => onPageChange(current_page - 1)}
          aria-label={text("COMMON.PREVIOUS")}
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>

        <span
          className="min-w-0 text-center font-medium tabular-nums"
          aria-current="page"
        >
          {text("COMMON.PAGE_POSITION", {
            args: [current_page, total_pages],
          })}
        </span>

        <Button
          variant="outline"
          size="icon"
          type="button"
          className="size-11 cursor-pointer"
          disabled={!has_next}
          onClick={() => onPageChange(current_page + 1)}
          aria-label={text("COMMON.NEXT")}
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      <div className="hidden items-center gap-1 sm:flex">
        <Button
          variant="outline"
          size="icon"
          type="button"
          className="size-11 cursor-pointer"
          disabled={!has_prev}
          onClick={() => onPageChange(current_page - 1)}
          aria-label={text("COMMON.PREVIOUS")}
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>

        {pages.map((p, i) =>
          p === 0 ? (
            <span key={`e${i}`} className="px-1 text-muted-foreground" aria-hidden="true">…</span>
          ) : (
            <Button
              key={p}
              variant={p === current_page ? "default" : "outline"}
              size="icon"
              type="button"
              className={cn("size-11 cursor-pointer", p === current_page && "pointer-events-none")}
              onClick={() => onPageChange(p)}
              aria-current={p === current_page ? "page" : undefined}
              aria-label={
                p === current_page
                  ? text("COMMON.CURRENT_PAGE", { args: [p] })
                  : text("COMMON.GO_TO_PAGE", { args: [p] })
              }
            >
              {p}
            </Button>
          )
        )}

        <Button
          variant="outline"
          size="icon"
          type="button"
          className="size-11 cursor-pointer"
          disabled={!has_next}
          onClick={() => onPageChange(current_page + 1)}
          aria-label={text("COMMON.NEXT")}
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </nav>
  );
}
