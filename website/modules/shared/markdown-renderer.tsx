"use client";

/**
 * Reusable markdown renderer for rendering simple markdown text.
 * Supports: headings (###), bullet lists (- ), inline code (`code`),
 * bold (**text**), and paragraph breaks.
 * Used across problem descriptions, editorial content, etc.
 */

interface MarkdownRendererProps {
  content: string;
  className?: string;
}

/** Renders inline markdown: `code` and **bold**. */
function InlineMarkdown({ text }: { text: string }) {
  // Split by backtick-wrapped code and bold patterns
  const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g);
  return (
    <>
      {parts.map((part, i) => {
        if (part.startsWith("`") && part.endsWith("`")) {
          return (
            <code
              key={i}
              className="px-1.5 py-0.5 rounded bg-muted text-primary text-sm font-mono"
            >
              {part.slice(1, -1)}
            </code>
          );
        }
        if (part.startsWith("**") && part.endsWith("**")) {
          return (
            <strong key={i} className="font-semibold text-foreground">
              {part.slice(2, -2)}
            </strong>
          );
        }
        return <span key={i}>{part}</span>;
      })}
    </>
  );
}

export function MarkdownRenderer({ content, className = "" }: MarkdownRendererProps) {
  const lines = content.split("\n");

  return (
    <div className={`prose dark:prose-invert max-w-none text-base leading-relaxed ${className}`}>
      {lines.map((line, i) => {
        if (line.startsWith("### ")) {
          return (
            <h3 key={i} className="text-lg font-bold mt-5 mb-2 text-foreground">
              <InlineMarkdown text={line.replace("### ", "")} />
            </h3>
          );
        }
        if (line.startsWith("- ")) {
          return (
            <p key={i} className="text-muted-foreground ml-5 my-1">
              <span className="text-primary mr-1">&#8226;</span>
              <InlineMarkdown text={line.replace("- ", "")} />
            </p>
          );
        }
        if (line.trim() === "") return <div key={i} className="h-2" />;
        return (
          <p key={i} className="text-muted-foreground my-1">
            <InlineMarkdown text={line} />
          </p>
        );
      })}
    </div>
  );
}
