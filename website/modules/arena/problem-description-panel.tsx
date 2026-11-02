"use client";

/**
 * Problem description panel — shows title, metadata badges, markdown, and test cases.
 */

import dynamic from "next/dynamic";
import { Badge } from "@/components/ui/badge";
import { TestCaseBlock } from "@/modules/problem/test-case-block";
import {
  DataLoadFeedback,
  DataLoadingFeedback,
} from "@/modules/problem/data-load-feedback";
import { TEXT } from "@/constants/text";
import { getDifficultyStyle } from "@/constants/styles";
import { formatDuration, formatMemory, formatNumber } from "@/lib/format";
import type { AsyncResourceStatus } from "@/hooks/use-retryable-resource";
import type { ProblemWorkspaceStatement } from "@/types/problem";
import type { TestCase } from "@/types/submission";
import {
  ChartNoAxesCombined,
  Clock,
  HardDrive,
  Loader2,
  Send,
} from "lucide-react";

const MarkdownRenderer = dynamic(
  () =>
    import("@/modules/shared/markdown-renderer").then(
      (module) => module.MarkdownRenderer,
    ),
  {
    loading: () => (
      <div
        className="flex min-h-32 items-center justify-center gap-2 rounded-xl border border-border bg-surface-sunken/30 text-sm text-muted-foreground"
        role="status"
      >
        <Loader2
          className="size-4 animate-spin motion-reduce:animate-none"
          aria-hidden="true"
        />
        {TEXT.PROBLEM.STATEMENT_RENDERING}
      </div>
    ),
  },
);

interface ProblemDescriptionPanelProps {
  problem: ProblemWorkspaceStatement;
  testCases: TestCase[];
  sampleStatus: AsyncResourceStatus;
  onRetrySamples: () => void;
}

export function ProblemDescriptionPanel({
  problem,
  testCases,
  sampleStatus,
  onRetrySamples,
}: ProblemDescriptionPanelProps) {
  const difficultyStyle = getDifficultyStyle(problem.difficulty?.level ?? 0);

  return (
    <div className="space-y-6">
      {/* Title + meta */}
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">{problem.title}</h1>
        <div className="flex items-center gap-3 flex-wrap">
          <Badge
            variant="outline"
            className={`px-3 py-1 text-sm font-bold ${difficultyStyle.bg} ${difficultyStyle.border} ${difficultyStyle.text}`}
          >
            {problem.difficulty?.name ?? TEXT.PROBLEM.DIFFICULTY_UNAVAILABLE}
          </Badge>

          {problem.tags && problem.tags.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {problem.tags.map((tag) => (
                <Badge
                  key={tag.id}
                  variant="secondary"
                  className="h-auto rounded-full border-0 bg-secondary px-2.5 py-1 text-xs font-medium text-secondary-foreground"
                >
                  {tag.name}
                </Badge>
              ))}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-x-3 gap-y-2 sm:ml-auto">
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <Clock className="h-4 w-4" aria-hidden="true" />
              {formatDuration(problem.time_limit_ms)}
            </span>
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <HardDrive className="h-4 w-4" aria-hidden="true" />
              {formatMemory(problem.memory_limit_kb)}
            </span>
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <ChartNoAxesCombined
                className="h-4 w-4"
                aria-hidden="true"
              />
              {TEXT.PROBLEM.ACCEPTANCE_RATE_LABEL(
                `${problem.acceptance_rate.toFixed(1)}%`,
              )}
            </span>
            <span className="flex items-center gap-1.5 text-sm text-muted-foreground">
              <Send className="h-4 w-4" aria-hidden="true" />
              {TEXT.PROBLEM.SUBMISSION_COUNT_LABEL(
                formatNumber(problem.submission_count),
              )}
            </span>
          </div>
        </div>
      </div>

      <div className="h-px bg-border/40" />

      {/* Problem description from API */}
      <MarkdownRenderer content={problem.description} />

      {sampleStatus === "loading" ? (
        <DataLoadingFeedback label={TEXT.PROBLEM.SAMPLES_LOADING} />
      ) : sampleStatus === "error" ? (
        <DataLoadFeedback
          title={TEXT.PROBLEM.SAMPLES_LOAD_ERROR_TITLE}
          description={TEXT.PROBLEM.SAMPLES_LOAD_ERROR_DESCRIPTION}
          retryLabel={TEXT.PROBLEM.RETRY}
          onRetry={onRetrySamples}
        />
      ) : testCases.length > 0 ? (
        <div className="space-y-3">
          <h3 className="text-lg font-bold text-foreground">
            {TEXT.PROBLEM.EXAMPLES}
          </h3>
          {testCases.map((tc, i) => (
            <TestCaseBlock key={i} testCase={tc} index={i} />
          ))}
        </div>
      ) : (
        <div className="rounded-xl border border-dashed border-border bg-surface-sunken/30 px-4 py-6 text-center text-sm text-muted-foreground">
          {TEXT.PROBLEM.SAMPLES_EMPTY}
        </div>
      )}
    </div>
  );
}
