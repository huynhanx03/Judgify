import { AlertTriangle, Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface DataLoadFeedbackProps {
  title: string;
  description?: string;
  retryLabel: string;
  onRetry: () => void;
  className?: string;
  compact?: boolean;
}

/** Recoverable, theme-token-only error state shared by problem experiences. */
export function DataLoadFeedback({
  title,
  description,
  retryLabel,
  onRetry,
  className,
  compact = false,
}: DataLoadFeedbackProps) {
  return (
    <div
      className={cn(
        "flex rounded-xl border border-destructive/25 bg-destructive/5 text-foreground",
        compact
          ? "flex-col items-stretch gap-3 p-3"
          : "flex-col items-start gap-4 p-5 sm:flex-row sm:items-center",
        className,
      )}
      role="alert"
      aria-live="polite"
    >
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-destructive/10 text-destructive">
          <AlertTriangle className="size-4" aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <p className="text-sm font-semibold text-foreground">{title}</p>
          {description ? (
            <p className="mt-1 text-sm leading-5 text-muted-foreground">
              {description}
            </p>
          ) : null}
        </div>
      </div>
      <Button
        type="button"
        variant="outline"
        className={cn(
          "h-11 shrink-0 cursor-pointer",
          compact && "w-full",
        )}
        onClick={onRetry}
      >
        <RefreshCw className="size-4" aria-hidden="true" />
        {retryLabel}
      </Button>
    </div>
  );
}

interface DataLoadingFeedbackProps {
  label: string;
  className?: string;
}

export function DataLoadingFeedback({
  label,
  className,
}: DataLoadingFeedbackProps) {
  return (
    <div
      className={cn(
        "flex min-h-24 items-center justify-center gap-3 rounded-xl border border-border/70 bg-muted/20 px-4 text-sm text-muted-foreground",
        className,
      )}
      role="status"
      aria-live="polite"
    >
      <Loader2
        className="size-4 animate-spin text-primary motion-reduce:animate-none"
        aria-hidden="true"
      />
      <span>{label}</span>
    </div>
  );
}
