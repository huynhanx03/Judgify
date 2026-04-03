/**
 * Difficulty badge component.
 * LeetCode-style indicator with a dot and vibrant colors.
 */

import { cn } from "@/lib/utils";
import type { DifficultyResponse } from "@/types/difficulty";

/** Style config per difficulty level. */
const LEVEL_STYLES: Record<number, string> = {
  1: "text-emerald-500 dark:text-emerald-400 drop-shadow-[0_0_2px_rgba(16,185,129,0.5)]",
  2: "text-amber-500 dark:text-amber-400 drop-shadow-[0_0_2px_rgba(245,158,11,0.5)]",
  3: "text-rose-500 dark:text-rose-400 drop-shadow-[0_0_2px_rgba(244,63,94,0.5)]",
};

interface DifficultyBadgeProps {
  difficulty?: DifficultyResponse;
  className?: string;
  showFormat?: "text-only" | "dot-text";
}

/** Renders a colored text badge indicating problem difficulty. */
export function DifficultyBadge({
  difficulty,
  className,
  showFormat = "dot-text",
}: DifficultyBadgeProps) {
  if (!difficulty) return <span className="text-muted-foreground text-sm">N/A</span>;

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-medium text-sm transition-colors",
        LEVEL_STYLES[difficulty.level] ?? LEVEL_STYLES[1],
        className
      )}
    >
      {showFormat === "dot-text" && (
        <span className="h-1.5 w-1.5 rounded-full bg-current shadow-[0_0_5px_currentColor]" />
      )}
      {difficulty.name}
    </span>
  );
}
