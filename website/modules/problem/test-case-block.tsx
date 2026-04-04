"use client";

/**
 * Test case display block with copy-to-clipboard buttons for input/output.
 * Used in problem detail page to show example test cases.
 */

import { useState } from "react";
import { TEXT } from "@/constants/text";
import type { TestCase } from "@/types/submission";
import { Copy, Check } from "lucide-react";

interface TestCaseBlockProps {
  testCase: TestCase;
  index: number;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <button
      onClick={handleCopy}
      className="text-muted-foreground hover:text-foreground transition-colors cursor-pointer p-1 rounded hover:bg-muted/50"
      title={copied ? TEXT.PROBLEM.COPIED : TEXT.PROBLEM.COPY}
    >
      {copied ? (
        <Check className="h-4 w-4 text-emerald-500" />
      ) : (
        <Copy className="h-4 w-4" />
      )}
    </button>
  );
}

export function TestCaseBlock({ testCase, index }: TestCaseBlockProps) {
  return (
    <div className="rounded-xl border border-border/40 bg-muted/20 overflow-hidden">
      <div className="px-4 py-2 border-b border-border/30 bg-muted/30">
        <span className="text-sm font-semibold text-muted-foreground">
          {TEXT.PROBLEM.EXAMPLE} {index + 1}
        </span>
      </div>
      <div className="p-4 space-y-3">
        {/* Input */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-xs uppercase tracking-wider font-semibold text-muted-foreground">
              {TEXT.PROBLEM.INPUT}
            </span>
            <CopyButton text={testCase.input} />
          </div>
          <pre className="p-3 rounded-lg bg-zinc-950 text-zinc-200 text-sm font-mono whitespace-pre overflow-x-auto leading-relaxed">
            {testCase.input}
          </pre>
        </div>

        {/* Output */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-xs uppercase tracking-wider font-semibold text-muted-foreground">
              {TEXT.PROBLEM.OUTPUT}
            </span>
            <CopyButton text={testCase.output} />
          </div>
          <pre className="p-3 rounded-lg bg-zinc-950 text-zinc-200 text-sm font-mono whitespace-pre overflow-x-auto leading-relaxed">
            {testCase.output}
          </pre>
        </div>

        {/* Explanation */}
        {testCase.explanation && (
          <p className="text-sm text-muted-foreground mt-2">
            <span className="font-semibold">{TEXT.PROBLEM.EXPLANATION}:</span>{" "}
            {testCase.explanation}
          </p>
        )}
      </div>
    </div>
  );
}
