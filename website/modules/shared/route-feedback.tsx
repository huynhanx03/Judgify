"use client";

import Link from "next/link";
import { CircleAlert, Home, RefreshCw, SearchX } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface RouteFeedbackProps {
  kind: "error" | "not-found";
  eyebrow: string;
  title: string;
  description: string;
  homeHref: string;
  homeLabel: string;
  retryLabel?: string;
  onRetry?: () => void;
}

/** Theme-safe terminal state shared by route-level error and not-found views. */
export function RouteFeedback({
  kind,
  eyebrow,
  title,
  description,
  homeHref,
  homeLabel,
  retryLabel,
  onRetry,
}: RouteFeedbackProps) {
  const Icon = kind === "error" ? CircleAlert : SearchX;

  return (
    <main className="relative flex min-h-[70dvh] items-center justify-center overflow-hidden px-4 py-16 sm:px-6">
      <div
        className="pointer-events-none absolute -top-32 size-80 rounded-full bg-primary/10 blur-3xl"
        aria-hidden="true"
      />
      <Card className="relative w-full max-w-xl overflow-hidden border-border/70 bg-card/90 shadow-xl backdrop-blur-sm">
        <CardContent className="flex flex-col items-center px-6 py-12 text-center sm:px-10">
          <div
            className={cn(
              "mb-6 flex size-16 items-center justify-center rounded-2xl ring-8",
              kind === "error"
                ? "bg-destructive/10 text-destructive ring-destructive/5"
                : "bg-primary/10 text-primary ring-primary/5",
            )}
          >
            <Icon className="size-7" aria-hidden="true" />
          </div>
          <p className="text-xs font-bold uppercase tracking-[0.2em] text-primary">
            {eyebrow}
          </p>
          <h1 className="mt-3 text-balance text-2xl font-black tracking-tight text-foreground sm:text-3xl">
            {title}
          </h1>
          <p className="mt-3 max-w-md text-pretty text-sm leading-6 text-muted-foreground sm:text-base">
            {description}
          </p>
          <div className="mt-7 flex w-full flex-col-reverse gap-3 sm:w-auto sm:flex-row">
            <Link
              href={homeHref}
              className={buttonVariants({
                variant: "outline",
                className: "w-full sm:w-auto",
              })}
            >
              <Home className="size-4" aria-hidden="true" />
              {homeLabel}
            </Link>
            {onRetry && retryLabel ? (
              <Button type="button" onClick={onRetry} className="w-full sm:w-auto">
                <RefreshCw className="size-4" aria-hidden="true" />
                {retryLabel}
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </main>
  );
}
