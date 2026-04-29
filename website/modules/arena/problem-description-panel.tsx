"use client";

/**
 * Problem description panel — shows title, metadata badges, markdown, and test cases.
 */

import { Badge } from "@/components/ui/badge";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";
import { TestCaseBlock } from "@/modules/problem/test-case-block";
import { TEXT } from "@/constants/text";
import type { Problem } from "@/types/problem";
import type { TestCase } from "@/types/submission";
import { Clock, HardDrive } from "lucide-react";

const DIFFICULTY_CONFIG: Record<number, { className: string }> = {
  1: { className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" },
  2: { className: "bg-amber-500/10 text-amber-500 border-amber-500/20" },
  3: { className: "bg-rose-500/10 text-rose-500 border-rose-500/20" },
};

interface ProblemDescriptionPanelProps {
  problem: Problem;
  testCases: TestCase[];
}

export function ProblemDescriptionPanel({ problem, testCases }: ProblemDescriptionPanelProps) {
  const diff = DIFFICULTY_CONFIG[problem.difficulty?.level ?? 1];

  return (
    <div className="space-y-6">
      {/* Title + meta */}
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">{problem.title}</h1>
        <div className="flex items-center gap-3 flex-wrap">
          <Badge
            variant="outline"
            className={`text-sm font-bold px-3 py-1 ${diff?.className ?? ""}`}
          >
            {problem.difficulty?.name ?? "N/A"}
          </Badge>

          {problem.tags && problem.tags.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {problem.tags.map((tag) => (
                <Badge
                  key={tag.id}
                  variant="secondary"
                  className="text-xs px-2.5 py-1 h-auto font-medium bg-violet-500/10 text-violet-600 dark:text-violet-400 border-0 rounded-full"
                >
                  {tag.name}
                </Badge>
              ))}
            </div>
          )}

          <div className="ml-auto flex items-center gap-3">
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <Clock className="h-4 w-4" />
              {problem.time_limit_ms} ms
            </span>
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <HardDrive className="h-4 w-4" />
              {(problem.memory_limit_kb / 1024).toFixed(0)} MB
            </span>
          </div>
        </div>
      </div>

      <div className="h-px bg-border/40" />

      {/* Problem description from API */}
      <MarkdownRenderer content={problem.description} />

      {/* Test cases from API */}
      {testCases.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-lg font-bold text-foreground">
            {TEXT.PROBLEM.EXAMPLES}
          </h3>
          {testCases.map((tc, i) => (
            <TestCaseBlock key={i} testCase={tc} index={i} />
          ))}
        </div>
      )}
    </div>
  );
}
