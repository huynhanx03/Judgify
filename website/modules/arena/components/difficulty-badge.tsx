/**
 * Difficulty badge component.
 * LeetCode-style indicator with a dot and vibrant colors.
 */

import { cn } from "@/lib/utils";
import { TEXT } from "@/constants/text";
import type { Difficulty } from "@/types/problem";

/** Maps difficulty to display text. */
const DIFFICULTY_LABELS: Record<Difficulty, string> = {
  easy: TEXT.ARENA.EASY,
  medium: TEXT.ARENA.MEDIUM,
  hard: TEXT.ARENA.HARD,
};

/** Maps difficulty to CSS classes for colored styling including text glow. */
const DIFFICULTY_STYLES: Record<Difficulty, string> = {
  easy: "text-emerald-500 dark:text-emerald-400 drop-shadow-[0_0_2px_rgba(16,185,129,0.5)]",
  medium: "text-amber-500 dark:text-amber-400 drop-shadow-[0_0_2px_rgba(245,158,11,0.5)]",
  hard: "text-rose-500 dark:text-rose-400 drop-shadow-[0_0_2px_rgba(244,63,94,0.5)]",
};

interface DifficultyBadgeProps {
  difficulty: Difficulty;
  className?: string;
  showFormat?: "text-only" | "dot-text";
}

/** Renders a colored text badge indicating problem difficulty. */
export function DifficultyBadge({
  difficulty,
  className,
  showFormat = "dot-text",
}: DifficultyBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-medium text-sm transition-colors",
        DIFFICULTY_STYLES[difficulty],
        className
      )}
    >
      {showFormat === "dot-text" && (
        <span className="h-1.5 w-1.5 rounded-full bg-current shadow-[0_0_5px_currentColor]" />
      )}
      {DIFFICULTY_LABELS[difficulty]}
    </span>
  );
}
