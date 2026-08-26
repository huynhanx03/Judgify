/**
 * Difficulty filter pills for the Knowledge Base.
 */

import { cn } from "@/lib/utils";
import { TEXT } from "@/constants/text";
import { getDifficultyStyle } from "@/constants/styles";
import { Loader2 } from "lucide-react";
import type { DifficultyResponse } from "@/types/difficulty";
import type { EntityID } from "@/types/api";

interface MaterialsDifficultyFilterProps {
  difficulties: DifficultyResponse[];
  active: EntityID | null;
  onChange: (difficultyId: EntityID | null) => void;
  loading?: boolean;
}

export function MaterialsDifficultyFilter({
  difficulties,
  active,
  onChange,
  loading = false,
}: MaterialsDifficultyFilterProps) {
  return (
    <div
      className="flex flex-wrap items-center gap-1.5"
      aria-busy={loading}
      aria-label={TEXT.MATERIALS.DIFFICULTY_FILTER_LABEL}
      role="group"
    >
      <button
        type="button"
        aria-pressed={active === null}
        onClick={() => onChange(null)}
        className={cn(
          "min-h-11 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
          active === null
            ? "bg-foreground text-background"
            : "text-muted-foreground hover:bg-muted hover:text-foreground",
        )}
      >
        {TEXT.MATERIALS.ALL}
      </button>

      {loading && difficulties.length === 0 ? (
        <p
          className="inline-flex min-h-11 items-center gap-2 px-2 text-xs text-muted-foreground"
          role="status"
          aria-live="polite"
        >
          <Loader2
            className="size-4 animate-spin motion-reduce:animate-none"
            aria-hidden="true"
          />
          {TEXT.MATERIALS.DIFFICULTIES_LOADING}
        </p>
      ) : null}

      {difficulties.map((difficulty) => {
        const isActive = active === difficulty.id;
        const style = getDifficultyStyle(difficulty.level);
        return (
          <button
            key={difficulty.id}
            type="button"
            aria-pressed={isActive}
            onClick={() => onChange(difficulty.id)}
            className={cn(
              "min-h-11 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
              isActive ? style.active : style.hover,
            )}
          >
            {difficulty.name}
          </button>
        );
      })}
    </div>
  );
}
