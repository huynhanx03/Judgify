"use client";

/**
 * Contest standings table — displays ICPC-style leaderboard data.
 */

import { Clock3, Swords } from "lucide-react";
import { TEXT } from "@/constants/text";
import type { Standing } from "@/types/contest";
import { RankIndicator } from "@/components/rank-indicator";

interface ContestStandingsTableProps {
  standings: Standing[];
}

export function ContestStandingsTable({ standings }: ContestStandingsTableProps) {
  if (standings.length === 0) {
    return (
      <div className="text-center py-16 text-muted-foreground">
        <Swords className="h-12 w-12 mx-auto mb-3 opacity-30" />
        <p className="text-lg font-medium">{TEXT.CONTEST.STANDINGS_EMPTY}</p>
        <p className="text-sm mt-1">{TEXT.CONTEST.STANDINGS_EMPTY_DESCRIPTION}</p>
      </div>
    );
  }

  const hasPending = standings.some(
    (standing) => standing.pending_attempts > 0,
  );

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <caption className="sr-only">{TEXT.CONTEST.STANDINGS}</caption>
        <thead>
          <tr className="border-b border-border/40">
            <th scope="col" className="text-left py-3 px-4 font-bold text-muted-foreground w-16">{TEXT.CONTEST.RANK}</th>
            <th scope="col" className="text-left py-3 px-4 font-bold text-muted-foreground">{TEXT.CONTEST.USER}</th>
            <th scope="col" className="text-center py-3 px-4 font-bold text-muted-foreground w-20">{TEXT.CONTEST.SOLVED}</th>
            <th scope="col" className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.PENALTY}</th>
            {hasPending ? (
              <th scope="col" className="text-center py-3 px-4 font-bold text-muted-foreground w-28">
                {TEXT.CONTEST.STANDINGS_PENDING}
              </th>
            ) : null}
          </tr>
        </thead>
        <tbody>
          {standings.map((s) => (
            <tr key={s.user_id} className="border-b border-border/20 hover:bg-muted/20 transition-colors">
              <td className="py-3 px-4">
                <RankIndicator rank={s.rank} />
              </td>
              <td className="py-3 px-4">
                <span className="font-medium">{s.username}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-bold text-success">{s.solved_count}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-mono text-muted-foreground">{s.penalty}</span>
              </td>
              {hasPending ? (
                <td className="py-3 px-4 text-center">
                  {s.pending_attempts > 0 ? (
                    <span
                      className="inline-flex items-center gap-1.5 rounded-full bg-warning/10 px-2 py-1 font-mono text-xs font-semibold text-warning"
                      title={TEXT.CONTEST.STANDINGS_PENDING_COUNT(
                        s.pending_attempts,
                      )}
                    >
                      <Clock3 className="size-3.5" aria-hidden="true" />
                      {s.pending_attempts}
                    </span>
                  ) : (
                    <span className="text-muted-foreground/50">—</span>
                  )}
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
