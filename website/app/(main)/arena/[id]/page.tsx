"use client";

/**
 * Problem detail page — full-width panel with two tabs.
 * Tab "De Bai": problem description (markdown) + visible test cases from API.
 * Tab "Lich Su": submission history from API (user's own submissions).
 * Submit: sends source code to backend via submission service.
 */

import { useState, useEffect, useCallback, use } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { getProblemById } from "@/services/problem.service";
import {
  createSubmission,
  getMySubmissions,
  getTestCases,
} from "@/services/submission.service";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";
import { FileSubmission } from "@/modules/problem/file-submission";
import { SubmissionHistory } from "@/modules/problem/submission-history";
import { TestCaseBlock } from "@/modules/problem/test-case-block";
import { TEXT } from "@/constants/text";
import { notify } from "@/lib/toast";
import type { Problem } from "@/types/problem";
import type {
  Submission,
  Language,
  TestCaseResponse,
  TestCase,
  SubmissionStatus,
} from "@/types/submission";
import { LoadingSpinner } from "@/components/loading-spinner";
import {
  ArrowLeft,
  Clock,
  HardDrive,
  FileText,
  History,
} from "lucide-react";

/** Style config per difficulty level (1=easy, 2=medium, 3=hard). */
const DIFFICULTY_CONFIG: Record<number, { className: string }> = {
  1: { className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" },
  2: { className: "bg-amber-500/10 text-amber-500 border-amber-500/20" },
  3: { className: "bg-rose-500/10 text-rose-500 border-rose-500/20" },
};

/** Terminal statuses — no further polling needed. */
const TERMINAL_STATUSES: SubmissionStatus[] = [
  "accepted",
  "wrong_answer",
  "time_limit_exceeded",
  "memory_limit_exceeded",
  "runtime_error",
  "compile_error",
];

/** Convert backend TestCaseResponse to display TestCase. */
function toDisplayTestCase(tc: TestCaseResponse): TestCase {
  return {
    input: tc.input,
    output: tc.expected_output,
  };
}

type Tab = "description" | "history";

export default function ProblemDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const problemId = Number(id);

  const [problem, setProblem] = useState<Problem | null>(null);
  const [isLoadingProblem, setIsLoadingProblem] = useState(true);
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [isLoadingSubmissions, setIsLoadingSubmissions] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>("description");

  // Fetch problem + test cases on mount
  useEffect(() => {
    async function loadProblem() {
      try {
        const [problemData, testCasesData] = await Promise.all([
          getProblemById(problemId),
          getTestCases(problemId),
        ]);
        setProblem(problemData);
        // Only show non-hidden test cases as examples
        const visible = testCasesData
          .filter((tc) => !tc.is_hidden)
          .sort((a, b) => a.order_index - b.order_index)
          .map(toDisplayTestCase);
        setTestCases(visible);
      } catch {
        // Problem will remain null -> shows "not found" state
      } finally {
        setIsLoadingProblem(false);
      }
    }
    loadProblem();
  }, [problemId]);

  // Fetch user's submission history for this problem
  const loadSubmissions = useCallback(async () => {
    setIsLoadingSubmissions(true);
    try {
      const data = await getMySubmissions(problemId);
      setSubmissions(data);
    } catch {
      setSubmissions([]);
    } finally {
      setIsLoadingSubmissions(false);
    }
  }, [problemId]);

  // Load submissions when switching to history tab
  useEffect(() => {
    if (activeTab === "history") {
      loadSubmissions();
    }
  }, [activeTab, loadSubmissions]);

  // Poll for submission result if latest is pending/judging
  useEffect(() => {
    const latest = submissions[0];
    if (!latest || TERMINAL_STATUSES.includes(latest.status)) return;

    const interval = setInterval(async () => {
      try {
        const data = await getMySubmissions(problemId);
        setSubmissions(data);
      } catch {
        // Silently retry
      }
    }, 2000);

    return () => clearInterval(interval);
  }, [submissions, problemId]);

  // Handle code submission
  async function handleSubmit(code: string, language: Language) {
    setIsSubmitting(true);
    setSubmitError(null);

    try {
      const newSubmission = await createSubmission({
        problem_id: problemId,
        language,
        source_code: code,
      });

      // Prepend new submission and switch to history tab
      setSubmissions((prev) => [newSubmission, ...prev]);
      setActiveTab("history");
      notify.success("Nộp bài thành công!");
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Nộp bài thất bại, thử lại sau.";
      setSubmitError(message);
    } finally {
      setIsSubmitting(false);
    }
  }

  // Loading state
  if (isLoadingProblem) return <LoadingSpinner />;

  // Not found state
  if (!problem) {
    return (
      <div className="text-center py-20 space-y-4">
        <p className="text-lg text-muted-foreground">
          {TEXT.PROBLEM.NOT_FOUND}
        </p>
        <Link href="/arena" className="text-primary hover:underline">
          {TEXT.PROBLEM.BACK_TO_ARENA}
        </Link>
      </div>
    );
  }

  const diff = DIFFICULTY_CONFIG[problem.difficulty?.level ?? 1];

  return (
    <div className="space-y-4 animate-in fade-in duration-300">
      {/* Error banner */}
      {submitError && (
        <div className="rounded-xl bg-red-500/10 px-5 py-3 text-base text-red-500 border border-red-500/20 animate-in fade-in">
          {submitError}
        </div>
      )}

      {/* Single full-width panel */}
      <Card className="glass-card border-border/40 overflow-hidden flex flex-col min-h-[calc(100vh-180px)]">
        {/* Top bar: back + submit */}
        <div className="px-6 py-3 border-b border-border/40 bg-muted/20 space-y-3">
          <div className="flex items-center justify-between gap-4">
            <Link
              href="/arena"
              className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors shrink-0"
            >
              <ArrowLeft className="h-4 w-4" />
              {TEXT.PROBLEM.BACK_TO_ARENA}
            </Link>
            <div className="flex-1">
              <FileSubmission
                onSubmit={handleSubmit}
                isSubmitting={isSubmitting}
              />
            </div>
          </div>
        </div>

        {/* Tab bar */}
        <div className="flex border-b border-border/40 bg-muted/10 px-2">
          <button
            onClick={() => setActiveTab("description")}
            className={`flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer ${
              activeTab === "description"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <FileText className="h-4 w-4" />
            {TEXT.PROBLEM.DESCRIPTION}
          </button>
          <button
            onClick={() => setActiveTab("history")}
            className={`flex items-center gap-2 px-5 py-3 text-sm font-semibold transition-colors cursor-pointer ${
              activeTab === "history"
                ? "text-primary border-b-2 border-primary"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <History className="h-4 w-4" />
            {TEXT.PROBLEM.SUBMISSION_HISTORY}
            {submissions.length > 0 && (
              <span className="text-[11px] bg-muted px-2 py-0.5 rounded-full font-bold">
                {submissions.length}
              </span>
            )}
          </button>
        </div>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto p-6">
          {activeTab === "description" && (
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
          )}

          {activeTab === "history" && (
            <SubmissionHistory
              submissions={submissions}
              isLoading={isLoadingSubmissions}
            />
          )}
        </div>
      </Card>
    </div>
  );
}
