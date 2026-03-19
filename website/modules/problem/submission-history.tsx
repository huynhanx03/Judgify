"use client";

/**
 * Submission history table — shows past submissions with verdict badges.
 */

import { Badge } from "@/components/ui/badge";
import type { Submission, Verdict } from "@/types/submission";

interface SubmissionHistoryProps {
  submissions: Submission[];
}

const VERDICT_CONFIG: Record<Verdict, { label: string; className: string }> = {
  accepted: { label: "Accepted", className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" },
  wrong_answer: { label: "Wrong Answer", className: "bg-red-500/10 text-red-500 border-red-500/20" },
  time_limit_exceeded: { label: "TLE", className: "bg-amber-500/10 text-amber-500 border-amber-500/20" },
  memory_limit_exceeded: { label: "MLE", className: "bg-purple-500/10 text-purple-500 border-purple-500/20" },
  runtime_error: { label: "Runtime Error", className: "bg-orange-500/10 text-orange-500 border-orange-500/20" },
  compilation_error: { label: "CE", className: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20" },
  pending: { label: "Judging...", className: "bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse" },
};

const LANG_LABELS: Record<string, string> = {
  cpp: "C++",
  java: "Java",
  python: "Python",
  go: "Go",
};

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function SubmissionHistory({ submissions }: SubmissionHistoryProps) {
  if (submissions.length === 0) {
    return (
      <div className="text-center py-8 text-sm text-muted-foreground">
        Chưa có lần nộp bài nào
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border/40 text-xs uppercase tracking-wider text-muted-foreground">
            <th className="text-left py-2 px-3 font-semibold">Kết Quả</th>
            <th className="text-left py-2 px-3 font-semibold">Ngôn Ngữ</th>
            <th className="text-right py-2 px-3 font-semibold">Thời Gian</th>
            <th className="text-right py-2 px-3 font-semibold">Bộ Nhớ</th>
            <th className="text-right py-2 px-3 font-semibold">Nộp Lúc</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/20">
          {submissions.map((s) => {
            const v = VERDICT_CONFIG[s.verdict];
            return (
              <tr key={s.id} className="hover:bg-muted/30 transition-colors">
                <td className="py-2.5 px-3">
                  <Badge variant="outline" className={`text-xs font-bold ${v.className}`}>
                    {v.label}
                  </Badge>
                </td>
                <td className="py-2.5 px-3 text-muted-foreground">
                  {LANG_LABELS[s.language] ?? s.language}
                </td>
                <td className="py-2.5 px-3 text-right font-mono text-xs">
                  {s.time_ms > 0 ? `${s.time_ms} ms` : "—"}
                </td>
                <td className="py-2.5 px-3 text-right font-mono text-xs">
                  {s.memory_kb > 0 ? `${(s.memory_kb / 1024).toFixed(1)} MB` : "—"}
                </td>
                <td className="py-2.5 px-3 text-right text-muted-foreground text-xs">
                  {formatTime(s.created_at)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
