import { Medal } from "lucide-react";
import { getPodiumPresentation } from "@/constants/ranking-presentation";
import { cn } from "@/lib/utils";

export function RankIndicator({ rank }: { rank: number }) {
  const podium = getPodiumPresentation(rank);

  if (!podium) {
    return <span className="font-semibold tabular-nums">{rank}</span>;
  }

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 font-bold tabular-nums",
        podium.text,
      )}
    >
      <Medal className="size-4" aria-hidden="true" />
      {rank}
    </span>
  );
}
