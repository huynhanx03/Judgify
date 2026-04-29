"use client";

/**
 * Contest standings table — displays ICPC-style leaderboard from SSE data.
 */

import { Swords } from "lucide-react";
import { TEXT } from "@/constants/text";
import type { Standing } from "@/types/contest";

interface ContestStandingsTableProps {
  standings: Standing[];
  isConnected: boolean;
  isActive: boolean;
}

export function ContestStandingsTable({ standings, isConnected, isActive }: ContestStandingsTableProps) {
  if (standings.length === 0) {
    return (
      <div className="text-center py-16 text-muted-foreground">
        <Swords className="h-12 w-12 mx-auto mb-3 opacity-30" />
        <p className="text-lg font-medium">Chưa có dữ liệu xếp hạng</p>
        <p className="text-sm mt-1">Bảng xếp hạng sẽ hiển thị khi cuộc thi bắt đầu.</p>
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
            <th className="text-center py-3 px-4 font-bold text-muted-foreground w-20">{TEXT.CONTEST.SOLVED}</th>
            <th className="text-center py-3 px-4 font-bold text-muted-foreground w-24">{TEXT.CONTEST.PENALTY}</th>
          </tr>
        </thead>
        <tbody>
          {standings.map((s) => (
            <tr key={s.user_id} className="border-b border-border/20 hover:bg-muted/20 transition-colors">
              <td className="py-3 px-4">
                <span className={`font-bold ${s.rank <= 3 ? "text-amber-500" : ""}`}>
                  {s.rank <= 3 ? ["🥇", "🥈", "🥉"][s.rank - 1] : s.rank}
                </span>
              </td>
              <td className="py-3 px-4">
                <span className="font-medium">{s.username}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-bold text-green-500">{s.solved_count}</span>
              </td>
              <td className="py-3 px-4 text-center">
                <span className="font-mono text-muted-foreground">{s.penalty}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
