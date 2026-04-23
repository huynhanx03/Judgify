/**
 * Difficulty filter pills for the Knowledge Base.
 */

import { cn } from "@/lib/utils";

const DIFFICULTIES = [
  { id: null, label: "Tất Cả" },
  { id: 1, label: "Nhập Môn" },
  { id: 2, label: "Cơ Bản" },
  { id: 3, label: "Nâng Cao" },
] as const;

const PILL_STYLES: Record<string, string> = {
  "Tất Cả": "data-[active=true]:bg-foreground data-[active=true]:text-background",
  "Nhập Môn": "data-[active=true]:bg-emerald-500/15 data-[active=true]:text-emerald-600 dark:data-[active=true]:text-emerald-400",
  "Cơ Bản": "data-[active=true]:bg-sky-500/15 data-[active=true]:text-sky-600 dark:data-[active=true]:text-sky-400",
  "Nâng Cao": "data-[active=true]:bg-amber-500/15 data-[active=true]:text-amber-600 dark:data-[active=true]:text-amber-400",
};

interface MaterialsDifficultyFilterProps {
  active: number | null;
  onChange: (difficultyId: number | null) => void;
}

export function MaterialsDifficultyFilter({ active, onChange }: MaterialsDifficultyFilterProps) {
  return (
    <div className="flex items-center gap-1.5 flex-wrap">
      {DIFFICULTIES.map((d) => {
        const isActive = active === d.id;
        return (
          <button
            key={d.label}
            data-active={isActive}
            onClick={() => onChange(d.id)}
            className={cn(
              "rounded-lg px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer",
              "text-muted-foreground hover:text-foreground hover:bg-muted",
              PILL_STYLES[d.label]
            )}
          >
            {d.label}
          </button>
        );
      })}
    </div>
  );
}
