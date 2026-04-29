"use client";

/**
 * Contest rating changes table — shows old/new rating and delta.
 */

import { TrendingUp } from "lucide-react";
import { TEXT } from "@/constants/text";

interface RatingChange {
  user_id: number;
  username: string;
  rank: number;
  old_rating: number;
  new_rating: number;
  delta: number;
}

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
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border/40">
            <th className="text-left py-3 px-4 font-bold text-muted-foreground w-16">{TEXT.CONTEST.RANK}</th>
            <th className="text-left py-3 px-4 font-bold text-muted-foreground">{TEXT.CONTEST.USER}</th>
            <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_OLD}</th>
            <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_NEW}</th>
            <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.RATING_DELTA}</th>
          </tr>
        </thead>
        <tbody>
          {ratingChanges.map((rc) => (
            <tr key={rc.user_id} className="border-b border-border/20 hover:bg-muted/20 transition-colors">
              <td className="py-3 px-4">
                <span className={`font-bold ${rc.rank <= 3 ? "text-amber-500" : ""}`}>
                  {rc.rank <= 3 ? ["🥇", "🥈", "🥉"][rc.rank - 1] : rc.rank}
                </span>
              </td>
              <td className="py-3 px-4">
                <span className="font-medium">{rc.username}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-mono text-muted-foreground">{rc.old_rating}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-mono font-bold">{rc.new_rating}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className={`font-mono font-bold ${rc.delta > 0 ? "text-green-500" : rc.delta < 0 ? "text-red-500" : "text-muted-foreground"}`}>
                  {rc.delta > 0 ? `+${rc.delta}` : rc.delta}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
