import { Medal } from "lucide-react";
import { cn } from "@/lib/utils";

const PODIUM_STYLES = [
  "text-amber-700 dark:text-amber-300",
  "text-slate-600 dark:text-slate-300",
  "text-orange-700 dark:text-orange-300",
] as const;

export function RankIndicator({ rank }: { rank: number }) {
  if (rank > PODIUM_STYLES.length) {
    return <span className="font-semibold tabular-nums">{rank}</span>;
  }

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 font-bold tabular-nums",
        PODIUM_STYLES[rank - 1],
      )}
    >
      <Medal className="size-4" aria-hidden="true" />
      {rank}
    </span>
  );
}
