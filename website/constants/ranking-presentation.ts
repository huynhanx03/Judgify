/**
 * Visual contract for ranked positions.
 *
 * Keep podium treatment here so contest tables, ranking cards, and any future
 * leaderboard use the same gold / silver / bronze language.  The first place
 * may inherit the leaderboard's semantic accent; second and third place stay
 * recognisable across every surface.
 */
export type RankingAccent = "amber" | "emerald";

export interface PodiumPresentation {
  border: string;
  background: string;
  text: string;
}

const SECOND_PLACE: PodiumPresentation = {
  border: "border-slate-400/40",
  background: "bg-slate-500/10",
  text: "text-slate-600 dark:text-slate-300",
};

const PODIUM_PRESENTATION: Record<RankingAccent, readonly PodiumPresentation[]> = {
  amber: [
    {
      border: "border-amber-500/60",
      background: "bg-amber-500/10",
      text: "text-amber-700 dark:text-amber-300",
    },
    SECOND_PLACE,
    {
      border: "border-orange-500/40",
      background: "bg-orange-500/10",
      text: "text-orange-700 dark:text-orange-300",
    },
  ],
  emerald: [
    {
      border: "border-emerald-500/60",
      background: "bg-emerald-500/10",
      text: "text-emerald-700 dark:text-emerald-300",
    },
    SECOND_PLACE,
    {
      border: "border-teal-500/40",
      background: "bg-teal-500/10",
      text: "text-teal-700 dark:text-teal-300",
    },
  ],
};

export const DEFAULT_RANKING_ACCENT: RankingAccent = "amber";

/** Returns the common visual treatment for ranks 1–3, otherwise undefined. */
export function getPodiumPresentation(
  rank: number,
  accent: RankingAccent = DEFAULT_RANKING_ACCENT,
): PodiumPresentation | undefined {
  return PODIUM_PRESENTATION[accent][rank - 1];
}
