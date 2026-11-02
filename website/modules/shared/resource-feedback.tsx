import { AlertTriangle, CircleAlert, Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface DataViewportProps {
  label: string;
  children: React.ReactNode;
  className?: string;
}

/**
 * The keyboard-accessible scroll boundary for data-dense content.
 *
 * It intentionally has no visual treatment: the receiving table, feed, or
 * state decides its own surface while all states share the same geometry.
 */
export function DataViewport({
  label,
  children,
  className,
}: DataViewportProps) {
  return (
    <div
      className={cn(
        "min-h-0 overflow-y-auto overscroll-contain outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
        className,
      )}
      role="region"
      aria-label={label}
      tabIndex={0}
    >
      {children}
    </div>
  );
}

interface ResourceCollectionFeedbackProps {
  title: string;
  description: string;
  retryLabel: string;
  onRetry: () => void;
  className?: string;
}

/** A centered recovery state that occupies a collection's existing content area. */
export function ResourceCollectionFeedback({
  title,
  description,
  retryLabel,
  onRetry,
  className,
}: ResourceCollectionFeedbackProps) {
  return (
    <div
      className={cn(
        "flex min-h-52 flex-col items-center justify-center px-6 py-12 text-center md:min-h-0 md:flex-1",
        className,
      )}
      role="alert"
      aria-live="polite"
    >
      <CircleAlert className="size-6 text-destructive" aria-hidden="true" />
      <h2 className="mt-4 text-lg font-semibold text-foreground">{title}</h2>
      <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
        {description}
      </p>
      <Button
        type="button"
        variant="outline"
        className="mt-6"
        onClick={onRetry}
      >
        <RefreshCw className="size-4" aria-hidden="true" />
        {retryLabel}
      </Button>
    </div>
  );
}

interface ResourceErrorFeedbackProps {
  title: string;
  description?: string;
  retryLabel: string;
  onRetry: () => void;
  className?: string;
  compact?: boolean;
}

/**
 * Recoverable resource feedback for every product surface.
 *
 * It deliberately keeps the retry control at a 44px target, preserves the
 * message hierarchy on narrow screens, and uses semantic theme tokens only.
 */
export function ResourceErrorFeedback({
  title,
  description,
  retryLabel,
  onRetry,
  className,
  compact = false,
}: ResourceErrorFeedbackProps) {
  return (
    <div
      className={cn(
        "flex border-l-2 border-destructive/60 bg-destructive/[0.03] text-foreground",
        compact
          ? "flex-col items-stretch gap-3 py-2 pl-3"
          : "flex-col items-start gap-4 py-3 pl-4 sm:flex-row sm:items-center",
        className,
      )}
      role="alert"
      aria-live="polite"
    >
      <div className="flex min-w-0 flex-1 items-start gap-2.5">
        <AlertTriangle
          className="mt-0.5 size-4 shrink-0 text-destructive"
          aria-hidden="true"
        />
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
        className={cn("h-11 shrink-0", compact && "w-full")}
        onClick={onRetry}
      >
        <RefreshCw className="size-4" aria-hidden="true" />
        {retryLabel}
      </Button>
    </div>
  );
}

interface ResourceLoadingFeedbackProps {
  label: string;
  className?: string;
  bare?: boolean;
}

/** A stable, live-announced loading state sized by the receiving surface. */
export function ResourceLoadingFeedback({
  label,
  className,
  bare = false,
}: ResourceLoadingFeedbackProps) {
  return (
    <div
      className={cn(
        "flex min-h-24 items-center justify-center gap-3 px-4 text-sm text-muted-foreground",
        !bare && "rounded-xl border border-border/70 bg-muted/20",
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
