import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

interface LoadingSpinnerProps {
  /** Wrapper className — override height/padding as needed. Default: h-[50vh] */
  className?: string;
}

/** Full-area loading spinner. Use className to control wrapper height/padding. */
export function LoadingSpinner({ className }: LoadingSpinnerProps) {
  return (
    <div className={cn("flex items-center justify-center", className ?? "h-[50vh]")}>
      <Loader2 className="h-8 w-8 animate-spin text-primary" />
    </div>
  );
}
