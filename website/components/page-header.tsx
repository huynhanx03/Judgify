import type { ComponentType, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  eyebrow?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  icon?: ComponentType<{ className?: string; "aria-hidden"?: boolean }>;
  className?: string;
}

/**
 * The shared heading hierarchy for product and operational page entry points.
 *
 * An icon is optional and deliberately unframed: page identity comes from the
 * heading, not a decorative card around it.
 */
export function PageHeader({
  title,
  eyebrow,
  description,
  actions,
  icon: Icon,
  className,
}: PageHeaderProps) {
  return (
    <header
      className={cn(
        "flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between",
        className,
      )}
      >
      <div className="min-w-0">
        {eyebrow ? (
          <div className="mb-1 text-xs font-semibold tracking-[0.18em] text-primary uppercase">
            {eyebrow}
          </div>
        ) : null}
        <div className="flex items-center gap-3">
          {Icon ? (
            <Icon className="size-6 shrink-0 text-primary" aria-hidden />
          ) : null}
          <h1 className="text-balance break-words text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            {title}
          </h1>
        </div>
        {description ? (
          <div className="mt-1 max-w-3xl text-sm leading-6 text-muted-foreground">
            {description}
          </div>
        ) : null}
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </header>
  );
}
