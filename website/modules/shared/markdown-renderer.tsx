"use client";

/**
 * Markdown renderer with LaTeX math support (KaTeX) + GFM.
 * Identical rendering to admin MDEditor preview.
 *
 * Syntax:
 *   Inline math:  $-10^9 \le a, b \le 10^9$
 *   Block math:   $$f(x) = x^2 + 2x + 1$$
 *   GFM: tables, strikethrough, autolinks, task lists, code blocks
 */

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

export function MarkdownRenderer({ content, className = "" }: MarkdownRendererProps) {
  return (
    <div className={`prose dark:prose-invert max-w-none text-base leading-relaxed ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={{
          code({ className, children, ...props }) {
            const isBlock = className?.startsWith("language-");
            if (isBlock) {
              return (
                <code
                  className={`${className} block bg-muted/50 rounded-lg p-4 overflow-x-auto text-sm font-mono`}
                  {...props}
                >
                  {children}
                </code>
              );
            }
            return (
              <code
                className="px-1.5 py-0.5 rounded bg-muted text-primary text-sm font-mono"
                {...props}
              >
                {children}
              </code>
            );
          },
          pre({ children }) {
            return <>{children}</>;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
