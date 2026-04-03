"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TEXT } from "@/constants/text";
import type { Submission, SubmissionStatus } from "@/types/submission";
import { Check, Copy, Loader2 } from "lucide-react";

interface SubmissionHistoryProps {
  submissions: Submission[];
  isLoading?: boolean;
}

const STATUS_CONFIG: Record<
  SubmissionStatus,
  { label: string; className: string }
> = {
  accepted: {
    label: "Accepted",
    className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20",
  },
  wrong_answer: {
    label: "Wrong Answer",
    className: "bg-red-500/10 text-red-500 border-red-500/20",
  },
  time_limit_exceeded: {
    label: "TLE",
    className: "bg-amber-500/10 text-amber-500 border-amber-500/20",
  },
  memory_limit_exceeded: {
    label: "MLE",
    className: "bg-purple-500/10 text-purple-500 border-purple-500/20",
  },
  runtime_error: {
    label: "Runtime Error",
    className: "bg-orange-500/10 text-orange-500 border-orange-500/20",
  },
  compile_error: {
    label: "CE",
    className: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20",
  },
  pending: {
    label: "Pending",
    className: "bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse",
  },
  judging: {
    label: "Judging...",
    className: "bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse",
  },
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

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function SubmissionDetail({ submission }: { submission: Submission }) {
  const [copied, setCopied] = useState(false);
  const cfg = STATUS_CONFIG[submission.status];

  function handleCopy() {
    navigator.clipboard.writeText(submission.source_code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="flex flex-col gap-4 overflow-y-auto px-4 pb-6">
      {/* Status + meta */}
      <div className="flex items-center gap-3 flex-wrap">
        <Badge variant="outline" className={`text-sm font-bold px-3 py-1 ${cfg.className}`}>
          {cfg.label}
        </Badge>
        <span className="text-sm text-muted-foreground">
          {LANG_LABELS[submission.language] ?? submission.language}
        </span>
        <span className="text-xs text-muted-foreground ml-auto">
          {formatDateTime(submission.created_at)}
        </span>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        <div className="rounded-lg border border-border/50 bg-muted/20 p-3 text-center">
          <p className="text-xs text-muted-foreground mb-1">{TEXT.PROBLEM.DETAIL_STATS_TESTS}</p>
          <p className="text-base font-bold font-mono">
            {submission.total_count > 0
              ? `${submission.passed_count}/${submission.total_count}`
              : "-"}
          </p>
        </div>
        <div className="rounded-lg border border-border/50 bg-muted/20 p-3 text-center">
          <p className="text-xs text-muted-foreground mb-1">{TEXT.PROBLEM.DETAIL_STATS_TIME}</p>
          <p className="text-base font-bold font-mono">
            {submission.time_ms != null ? `${submission.time_ms} ms` : "-"}
          </p>
        </div>
        <div className="rounded-lg border border-border/50 bg-muted/20 p-3 text-center">
          <p className="text-xs text-muted-foreground mb-1">{TEXT.PROBLEM.DETAIL_STATS_MEMORY}</p>
          <p className="text-base font-bold font-mono">
            {submission.memory_kb != null
              ? `${(submission.memory_kb / 1024).toFixed(1)} MB`
              : "-"}
          </p>
        </div>
      </div>

      {/* Error message */}
      {submission.error_message && (
        <>
          <Separator />
          <div>
            <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
              {TEXT.PROBLEM.DETAIL_ERROR}
            </p>
            <pre className="rounded-lg bg-red-500/5 border border-red-500/20 p-3 text-xs text-red-400 font-mono overflow-x-auto whitespace-pre-wrap break-all">
              {submission.error_message}
            </pre>
          </div>
        </>
      )}

      <Separator />

      {/* Source code */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            {TEXT.PROBLEM.DETAIL_SOURCE_CODE}
          </p>
          <Button variant="ghost" size="sm" className="h-7 gap-1.5 text-xs" onClick={handleCopy}>
            {copied ? (
              <><Check className="h-3.5 w-3.5" />{TEXT.PROBLEM.DETAIL_COPIED}</>
            ) : (
              <><Copy className="h-3.5 w-3.5" />{TEXT.PROBLEM.DETAIL_COPY}</>
            )}
          </Button>
        </div>
        <pre className="rounded-lg bg-muted/30 border border-border/50 p-3 text-xs font-mono overflow-x-auto whitespace-pre leading-relaxed">
          {submission.source_code}
        </pre>
      </div>
    </div>
  );
}

export function SubmissionHistory({
  submissions,
  isLoading = false,
}: SubmissionHistoryProps) {
  const [selected, setSelected] = useState<Submission | null>(null);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-primary" />
        <span className="ml-2 text-sm text-muted-foreground">
          {TEXT.PROBLEM.LOADING_SUBMISSIONS}
        </span>
      </div>
    );
  }

  if (submissions.length === 0) {
    return (
      <div className="text-center py-8 text-sm text-muted-foreground">
        {TEXT.PROBLEM.NO_SUBMISSIONS}
      </div>
    );
  }

  return (
    <>
      <div className="rounded-lg border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-muted/30">
              <TableHead className="w-[150px]">{TEXT.PROBLEM.COL_RESULT}</TableHead>
              <TableHead className="w-[90px]">{TEXT.PROBLEM.COL_LANGUAGE}</TableHead>
              <TableHead className="w-[70px] text-center">{TEXT.PROBLEM.COL_TESTS}</TableHead>
              <TableHead className="w-[100px] text-right">{TEXT.PROBLEM.COL_TIME}</TableHead>
              <TableHead className="w-[90px] text-right">{TEXT.PROBLEM.COL_MEMORY}</TableHead>
              <TableHead className="text-right">{TEXT.PROBLEM.COL_SUBMITTED_AT}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {submissions.map((s) => {
              const cfg = STATUS_CONFIG[s.status];
              return (
                <TableRow
                  key={s.id}
                  className="hover:bg-muted/20 cursor-pointer"
                  onClick={() => setSelected(s)}
                >
                  <TableCell>
                    <Badge
                      variant="outline"
                      className={`text-xs font-bold whitespace-nowrap ${cfg.className}`}
                    >
                      {cfg.label}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {LANG_LABELS[s.language] ?? s.language}
                  </TableCell>
                  <TableCell className="text-center font-mono text-xs">
                    {s.total_count > 0
                      ? `${s.passed_count}/${s.total_count}`
                      : "-"}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {s.time_ms != null ? `${s.time_ms} ms` : "-"}
                  </TableCell>
                  <TableCell className="text-right font-mono text-xs">
                    {s.memory_kb != null
                      ? `${(s.memory_kb / 1024).toFixed(1)} MB`
                      : "-"}
                  </TableCell>
                  <TableCell className="text-right text-muted-foreground text-xs">
                    {formatTime(s.created_at)}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <Sheet open={selected !== null} onOpenChange={(open) => !open && setSelected(null)}>
        <SheetContent side="right" className="w-full sm:max-w-2xl overflow-hidden p-0">
          <SheetHeader className="px-4 pt-4 pb-3 border-b border-border/50">
            <SheetTitle>{TEXT.PROBLEM.DETAIL_TITLE}</SheetTitle>
          </SheetHeader>
          {selected && <SubmissionDetail submission={selected} />}
        </SheetContent>
      </Sheet>
    </>
  );
}
