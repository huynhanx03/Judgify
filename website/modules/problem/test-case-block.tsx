"use client";

/**
 * Test case display block with copy-to-clipboard buttons for input/output.
 * Used in problem detail page to show example test cases.
 */

import { TEXT } from "@/constants/text";
import { CopyButton } from "@/modules/problem/copy-button";
import type { TestCase } from "@/types/submission";

interface TestCaseBlockProps {
  testCase: TestCase;
  index: number;
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
            <CopyButton
              value={testCase.input}
              idleLabel={TEXT.PROBLEM.COPY_INPUT}
              successLabel={TEXT.PROBLEM.COPIED}
              errorLabel={TEXT.PROBLEM.COPY_FAILED}
            />
          </div>
          <pre className="overflow-x-auto rounded-lg bg-code-background p-3 font-mono text-sm leading-relaxed whitespace-pre text-media-foreground">
            {testCase.input}
          </pre>
        </div>

        {/* Output */}
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-xs uppercase tracking-wider font-semibold text-muted-foreground">
              {TEXT.PROBLEM.OUTPUT}
            </span>
            <CopyButton
              value={testCase.output}
              idleLabel={TEXT.PROBLEM.COPY_OUTPUT}
              successLabel={TEXT.PROBLEM.COPIED}
              errorLabel={TEXT.PROBLEM.COPY_FAILED}
            />
          </div>
          <pre className="overflow-x-auto rounded-lg bg-code-background p-3 font-mono text-sm leading-relaxed whitespace-pre text-media-foreground">
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
