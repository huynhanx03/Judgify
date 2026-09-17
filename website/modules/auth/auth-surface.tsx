import type { ComponentProps, ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface AuthSurfaceProps extends ComponentProps<typeof Card> {
  children: ReactNode;
  framed?: boolean;
}

/**
 * One opaque, theme-aware shell for every authentication journey.
 *
 * Auth forms contain sensitive, high-focus work, so this intentionally avoids
 * translucent glass and lets the surface token own contrast in both themes.
 */
export function AuthSurface({
  children,
  className,
  framed = false,
  ...props
}: AuthSurfaceProps) {
  return (
    <Card
      className={cn(
        "relative w-full border-border/80 bg-surface shadow-2xl",
        className,
      )}
      {...props}
    >
      {framed ? <AuthFrame /> : null}
      {children}
    </Card>
  );
}

function AuthFrame() {
  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0">
      <span className="absolute left-0 top-0 size-16 rounded-tl-xl border-l-2 border-t-2 border-primary/30" />
      <span className="absolute right-0 top-0 size-16 rounded-tr-xl border-r-2 border-t-2 border-primary/30" />
      <span className="absolute bottom-0 left-0 size-16 rounded-bl-xl border-b-2 border-l-2 border-primary/30" />
      <span className="absolute bottom-0 right-0 size-16 rounded-br-xl border-b-2 border-r-2 border-primary/30" />
    </div>
  );
}
