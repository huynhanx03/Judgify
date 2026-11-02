import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { text } from "@/i18n/text";

interface LoadingSpinnerProps {
  /** Wrapper className — override height/padding as needed. Default: h-[50vh] */
  className?: string;
  label?: string;
}

/** Full-area loading spinner. Use className to control wrapper height/padding. */
export function LoadingSpinner({
  className,
  label = text("COMMON.LOADING"),
}: LoadingSpinnerProps) {
  return (
    <div
      className={cn("flex items-center justify-center", className ?? "h-[50vh]")}
      role="status"
      aria-live="polite"
    >
      <Loader2 className="size-8 animate-spin text-primary motion-reduce:animate-none" aria-hidden="true" />
      <span className="sr-only">{label}</span>
    </div>
  );
}
