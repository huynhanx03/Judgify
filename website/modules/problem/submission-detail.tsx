"use client";

/**
 * Submission detail panel — shown in a Sheet when a row is clicked.
 * Contains STATUS_CONFIG and LANG_LABELS since they're only used here.
 */

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { TEXT } from "@/constants/text";
import { formatDateTime } from "@/lib/format";
import { Check, Copy } from "lucide-react";
import type { Submission, SubmissionStatus } from "@/types/submission";

export const STATUS_CONFIG: Record<SubmissionStatus, { label: string; className: string }> = {
  accepted:             { label: "Accepted",      className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" },
  wrong_answer:         { label: "Wrong Answer",  className: "bg-red-500/10 text-red-500 border-red-500/20" },
  time_limit_exceeded:  { label: "TLE",           className: "bg-amber-500/10 text-amber-500 border-amber-500/20" },
  memory_limit_exceeded:{ label: "MLE",           className: "bg-purple-500/10 text-purple-500 border-purple-500/20" },
  runtime_error:        { label: "Runtime Error", className: "bg-orange-500/10 text-orange-500 border-orange-500/20" },
  compile_error:        { label: "CE",            className: "bg-zinc-500/10 text-zinc-400 border-zinc-500/20" },
  pending:              { label: "Pending",       className: "bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse" },
  judging:              { label: "Judging...",    className: "bg-blue-500/10 text-blue-400 border-blue-500/20 animate-pulse" },
};

export const LANG_LABELS: Record<string, string> = {
  cpp: "C++", java: "Java", python: "Python", go: "Go",
};

interface SubmissionDetailProps {
  submission: Submission;
}

export function SubmissionDetail({ submission }: SubmissionDetailProps) {
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
            {submission.total_count > 0 ? `${submission.passed_count}/${submission.total_count}` : "-"}
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
            {submission.memory_kb != null ? `${(submission.memory_kb / 1024).toFixed(1)} MB` : "-"}
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
