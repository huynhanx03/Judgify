"use client";

/**
 * Reusable pagination component with page number buttons.
 * Shows: prev, page numbers (with ellipsis), next, and total items info.
 */

import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import type { PaginationMeta } from "@/types/api";
import { cn } from "@/lib/utils";

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
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground">
        Tổng {total_items} mục
      </span>
      <div className="flex items-center gap-1">
        <Button
          variant="outline"
          size="icon"
          className="h-8 w-8 cursor-pointer"
          disabled={!has_prev}
          onClick={() => onPageChange(current_page - 1)}
        >
          <ChevronLeft className="h-4 w-4" />
        </Button>

        {pages.map((p, i) =>
          p === 0 ? (
            <span key={`e${i}`} className="px-1 text-muted-foreground">…</span>
          ) : (
            <Button
              key={p}
              variant={p === current_page ? "default" : "outline"}
              size="icon"
              className={cn("h-8 w-8 cursor-pointer", p === current_page && "pointer-events-none")}
              onClick={() => onPageChange(p)}
            >
              {p}
            </Button>
          )
        )}

        <Button
          variant="outline"
          size="icon"
          className="h-8 w-8 cursor-pointer"
          disabled={!has_next}
          onClick={() => onPageChange(current_page + 1)}
        >
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
