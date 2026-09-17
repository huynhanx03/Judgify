/**
 * Difficulty badge component.
 * LeetCode-style indicator with a dot and vibrant colors.
 */

import { cn } from "@/lib/utils";
import { getDifficultyStyle } from "@/constants/styles";
import { TEXT } from "@/constants/text";
import type { DifficultyResponse } from "@/types/difficulty";

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
  if (!difficulty) {
    return (
      <span className="text-muted-foreground text-sm">
        {TEXT.PROBLEM.DIFFICULTY_UNAVAILABLE}
      </span>
    );
  }

  const style = getDifficultyStyle(difficulty.level);

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-medium text-sm transition-colors",
        style.text,
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
