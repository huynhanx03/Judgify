/**
 * Difficulty filter pills for the Knowledge Base.
 */

import type { MaterialDifficulty } from "@/types/material";
import { cn } from "@/lib/utils";

const DIFFICULTIES: (MaterialDifficulty | "Tất Cả")[] = [
  "Tất Cả",
  "Nhập Môn",
  "Cơ Bản",
  "Nâng Cao",
  "Chuyên Sâu",
];

const PILL_STYLES: Record<string, string> = {
  "Tất Cả": "data-[active=true]:bg-foreground data-[active=true]:text-background",
  "Nhập Môn": "data-[active=true]:bg-emerald-500/15 data-[active=true]:text-emerald-600 dark:data-[active=true]:text-emerald-400",
  "Cơ Bản": "data-[active=true]:bg-sky-500/15 data-[active=true]:text-sky-600 dark:data-[active=true]:text-sky-400",
  "Nâng Cao": "data-[active=true]:bg-amber-500/15 data-[active=true]:text-amber-600 dark:data-[active=true]:text-amber-400",
  "Chuyên Sâu": "data-[active=true]:bg-rose-500/15 data-[active=true]:text-rose-600 dark:data-[active=true]:text-rose-400",
};

interface MaterialsDifficultyFilterProps {
  active: MaterialDifficulty | null;
  onChange: (difficulty: MaterialDifficulty | null) => void;
}

export function MaterialsDifficultyFilter({ active, onChange }: MaterialsDifficultyFilterProps) {
  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {DIFFICULTIES.map((d) => {
        const isActive = d === "Tất Cả" ? active === null : active === d;
        return (
          <button
            key={d}
            data-active={isActive}
            onClick={() => onChange(d === "Tất Cả" ? null : (d as MaterialDifficulty))}
            className={cn(
              "rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer",
              "text-muted-foreground hover:text-foreground hover:bg-muted",
              PILL_STYLES[d]
            )}
          >
            {d}
          </button>
        );
      })}
    </div>
  );
}
