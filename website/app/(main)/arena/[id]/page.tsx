"use client";

/**
 * Problem detail page — single full-width panel with two tabs.
 * Top bar: back link, title, difficulty, tags, constraints, submit area — all inside panel.
 * Tab "Đề Bài": markdown description + test cases (full width).
 * Tab "Lịch Sử": submission history table.
 */

import { useState, useEffect, use } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { getProblemById } from "@/services/problem.service";
import { MOCK_TEST_CASES, MOCK_SUBMISSIONS, MOCK_PROBLEM_DESCRIPTION } from "@/mock/submissions";
import { MarkdownRenderer } from "@/modules/shared/markdown-renderer";
import { FileSubmission } from "@/modules/problem/file-submission";
import { SubmissionHistory } from "@/modules/problem/submission-history";
import { TestCaseBlock } from "@/modules/problem/test-case-block";
import { TEXT } from "@/constants/text";
import type { Problem } from "@/types/problem";
import type { Submission, Language } from "@/types/submission";
import {
  Loader2,
  ArrowLeft,
  Clock,
  HardDrive,
  CheckCircle2,
  FileText,
  History,
} from "lucide-react";

const DIFFICULTY_CONFIG = {
  easy: { label: TEXT.ARENA.EASY, className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" },
  medium: { label: TEXT.ARENA.MEDIUM, className: "bg-amber-500/10 text-amber-500 border-amber-500/20" },
  hard: { label: TEXT.ARENA.HARD, className: "bg-rose-500/10 text-rose-500 border-rose-500/20" },
};

type Tab = "description" | "history";

export default function ProblemDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [submissions, setSubmissions] = useState<Submission[]>(MOCK_SUBMISSIONS);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitSuccess, setSubmitSuccess] = useState(false);
  const [activeTab, setActiveTab] = useState<Tab>("description");

  useEffect(() => {
    getProblemById(Number(id)).then((data) => {
      setProblem(data);
      setIsLoading(false);
    });
  }, [id]);

  async function handleSubmit(code: string, language: Language) {
    setIsSubmitting(true);
    setSubmitSuccess(false);

    await new Promise((resolve) => setTimeout(resolve, 1500));

    const verdicts = ["accepted", "wrong_answer", "accepted", "accepted"] as const;
    const verdict = verdicts[Math.floor(Math.random() * verdicts.length)];

    const newSubmission: Submission = {
      id: Date.now(),
      problem_id: Number(id),
      language,
      verdict,
      time_ms: verdict === "accepted" ? Math.floor(Math.random() * 50) + 5 : Math.floor(Math.random() * 200),
      memory_kb: Math.floor(Math.random() * 10000) + 3000,
      created_at: new Date().toISOString(),
    };

    setSubmissions((prev) => [newSubmission, ...prev]);
    setIsSubmitting(false);
    setActiveTab("history");
    if (verdict === "accepted") setSubmitSuccess(true);
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 className="h-10 w-10 animate-spin text-primary" />
      </div>
    );
  }

  if (!problem) {
    return (
      <div className="text-center py-20 space-y-4">
        <p className="text-lg text-muted-foreground">{TEXT.PROBLEM.NOT_FOUND}</p>
        <Link href="/arena" className="text-primary hover:underline">
          {TEXT.PROBLEM.BACK_TO_ARENA}
        </Link>
      </div>
    );
  }

  const diff = DIFFICULTY_CONFIG[problem.difficulty];

  return (
    <div className="space-y-4 animate-in fade-in duration-300">
      {/* Success banner */}
      {submitSuccess && (
        <div className="rounded-xl bg-emerald-500/10 px-5 py-3 text-base text-emerald-500 border border-emerald-500/20 animate-in fade-in flex items-center gap-2">
          <CheckCircle2 className="h-5 w-5" />
          {TEXT.PROBLEM.SUBMIT_SUCCESS}
        </div>
      )}

      {/* Single full-width panel */}
      <Card className="glass-card border-border/40 overflow-hidden flex flex-col min-h-[calc(100vh-180px)]">
        {/* Top bar: back + submit + tabs */}
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
              <FileSubmission onSubmit={handleSubmit} isSubmitting={isSubmitting} />
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

        {/* Tab content — full width, no max-w constraint */}
        <div className="flex-1 overflow-y-auto p-6">
          {activeTab === "description" && (
            <div className="space-y-6">
              {/* Title + meta inside description */}
              <div className="space-y-3">
                <h1 className="text-2xl font-bold">{problem.title}</h1>
                <div className="flex items-center gap-3 flex-wrap">
                  <Badge variant="outline" className={`text-sm font-bold px-3 py-1 ${diff.className}`}>
                    {diff.label}
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

                  {/* Push time/memory to the right */}
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

              <MarkdownRenderer content={MOCK_PROBLEM_DESCRIPTION} />

              {/* Test cases */}
              <div className="space-y-3">
                <h3 className="text-lg font-bold text-foreground">{TEXT.PROBLEM.EXAMPLES}</h3>
                {MOCK_TEST_CASES.map((tc, i) => (
                  <TestCaseBlock key={i} testCase={tc} index={i} />
                ))}
              </div>
            </div>
          )}

          {activeTab === "history" && (
            <SubmissionHistory submissions={submissions} />
          )}
        </div>
      </Card>
    </div>
  );
}
