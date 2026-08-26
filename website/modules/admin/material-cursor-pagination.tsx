"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { text } from "@/i18n/text";

export function MaterialCursorPagination({
  page,
  hasPrevious,
  hasNext,
  onPageChange,
}: {
  page: number;
  hasPrevious: boolean;
  hasNext: boolean;
  onPageChange: (page: number) => void;
}) {
  if (!hasPrevious && !hasNext) return null;
  return (
    <nav
      className="flex items-center justify-end gap-3"
      aria-label={text("COMMON.PAGINATION")}
    >
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="size-11 cursor-pointer"
        disabled={!hasPrevious}
        onClick={() => onPageChange(page - 1)}
        aria-label={text("COMMON.PREVIOUS")}
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </Button>
      <span className="min-w-16 text-center text-sm font-medium tabular-nums">
        {text("COMMON.CURRENT_PAGE", { args: [page] })}
      </span>
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="size-11 cursor-pointer"
        disabled={!hasNext}
        onClick={() => onPageChange(page + 1)}
        aria-label={text("COMMON.NEXT")}
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </Button>
    </nav>
  );
}
