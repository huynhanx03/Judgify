"use client";

/**
 * Contest standings table — displays ICPC-style leaderboard data.
 */

import { Clock3, Swords } from "lucide-react";
import { TEXT } from "@/constants/text";
import type { Standing } from "@/types/contest";
import { RankIndicator } from "@/components/rank-indicator";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

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
    <Table containerLabel={TEXT.CONTEST.STANDINGS}>
        <TableCaption className="sr-only">{TEXT.CONTEST.STANDINGS}</TableCaption>
        <TableHeader>
          <TableRow className="border-border/40">
            <TableHead className="w-16 px-4 py-3 text-left text-sm font-bold">{TEXT.CONTEST.RANK}</TableHead>
            <TableHead className="px-4 py-3 text-left text-sm font-bold">{TEXT.CONTEST.USER}</TableHead>
            <TableHead className="w-20 px-4 py-3 text-center text-sm font-bold">{TEXT.CONTEST.SOLVED}</TableHead>
            <TableHead className="w-24 px-4 py-3 text-center text-sm font-bold">{TEXT.CONTEST.PENALTY}</TableHead>
            {hasPending ? (
              <TableHead className="w-28 px-4 py-3 text-center text-sm font-bold">
                {TEXT.CONTEST.STANDINGS_PENDING}
              </TableHead>
            ) : null}
          </TableRow>
        </TableHeader>
        <TableBody>
          {standings.map((s) => (
            <TableRow key={s.user_id} className="border-border/20 hover:bg-muted/20">
              <TableCell className="px-4 py-3">
                <RankIndicator rank={s.rank} />
              </TableCell>
              <TableCell className="px-4 py-3">
                <span className="font-medium">{s.username}</span>
              </TableCell>
              <TableCell className="px-4 py-3 text-center">
                <span className="font-bold text-success">{s.solved_count}</span>
              </TableCell>
              <TableCell className="px-4 py-3 text-center">
                <span className="font-mono text-muted-foreground">{s.penalty}</span>
              </TableCell>
              {hasPending ? (
                <TableCell className="px-4 py-3 text-center">
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
                </TableCell>
              ) : null}
            </TableRow>
          ))}
        </TableBody>
    </Table>
  );
}
