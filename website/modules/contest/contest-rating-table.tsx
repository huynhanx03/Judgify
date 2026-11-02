"use client";

/**
 * Contest rating changes table — shows old/new rating and delta.
 */

import { TrendingUp } from "lucide-react";
import { TEXT } from "@/constants/text";
import type { RatingChange } from "@/types/contest";
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

interface ContestRatingTableProps {
  ratingChanges: RatingChange[];
}

export function ContestRatingTable({ ratingChanges }: ContestRatingTableProps) {
  if (ratingChanges.length === 0) {
    return (
      <div className="text-center py-16 text-muted-foreground">
        <TrendingUp className="h-12 w-12 mx-auto mb-3 opacity-30" />
        <p className="text-lg font-medium">{TEXT.CONTEST.NO_RATING_CHANGES}</p>
        <p className="text-sm mt-1">{TEXT.CONTEST.NO_RATING_CHANGES_DESC}</p>
      </div>
    );
  }

  return (
    <Table containerLabel={TEXT.CONTEST.RATING_CHANGES}>
        <TableCaption className="sr-only">{TEXT.CONTEST.RATING_CHANGES}</TableCaption>
        <TableHeader>
          <TableRow className="border-border/40">
            <TableHead className="w-16 px-4 py-3 text-left text-sm font-bold">{TEXT.CONTEST.RANK}</TableHead>
            <TableHead className="px-4 py-3 text-left text-sm font-bold">{TEXT.CONTEST.USER}</TableHead>
            <TableHead className="w-24 px-4 py-3 text-center text-sm font-bold">{TEXT.CONTEST.RATING_OLD}</TableHead>
            <TableHead className="w-24 px-4 py-3 text-center text-sm font-bold">{TEXT.CONTEST.RATING_NEW}</TableHead>
            <TableHead className="w-24 px-4 py-3 text-center text-sm font-bold">{TEXT.CONTEST.RATING_DELTA}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {ratingChanges.map((rc) => (
            <TableRow key={rc.user_id} className="border-border/20 hover:bg-muted/20">
              <TableCell className="px-4 py-3">
                <RankIndicator rank={rc.rank} />
              </TableCell>
              <TableCell className="px-4 py-3">
                <span className="font-medium">{rc.username}</span>
              </TableCell>
              <TableCell className="px-4 py-3 text-center">
                <span className="font-mono text-muted-foreground">{rc.old_rating}</span>
              </TableCell>
              <TableCell className="px-4 py-3 text-center">
                <span className="font-mono font-bold">{rc.new_rating}</span>
              </TableCell>
              <TableCell className="px-4 py-3 text-center">
                <span className={`font-mono font-bold ${rc.delta > 0 ? "text-success" : rc.delta < 0 ? "text-danger" : "text-muted-foreground"}`}>
                  {rc.delta > 0 ? `+${rc.delta}` : rc.delta}
                </span>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
    </Table>
  );
}
