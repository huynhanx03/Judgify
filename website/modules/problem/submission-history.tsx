"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
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
import { formatTime } from "@/lib/format";
import { LoadingSpinner } from "@/components/loading-spinner";
import { SubmissionDetail, STATUS_CONFIG, LANG_LABELS } from "@/modules/problem/submission-detail";
import type { Submission } from "@/types/submission";

interface SubmissionHistoryProps {
  submissions: Submission[];
  isLoading?: boolean;
}

export function SubmissionHistory({
  submissions,
  isLoading = false,
}: SubmissionHistoryProps) {
  const [selected, setSelected] = useState<Submission | null>(null);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <LoadingSpinner className="py-0" />
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
