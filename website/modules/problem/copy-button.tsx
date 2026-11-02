"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Copy, TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PROBLEM_UI_POLICY } from "@/constants/problem";
import { cn } from "@/lib/utils";

type CopyState = "idle" | "copied" | "error";

interface CopyButtonProps {
  value: string;
  idleLabel: string;
  successLabel: string;
  errorLabel: string;
  showLabel?: boolean;
  className?: string;
}

/** Accessible clipboard action with truthful success/error feedback. */
export function CopyButton({
  value,
  idleLabel,
  successLabel,
  errorLabel,
  showLabel = false,
  className,
}: CopyButtonProps) {
  const [state, setState] = useState<CopyState>("idle");
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const label =
    state === "copied"
      ? successLabel
      : state === "error"
        ? errorLabel
        : idleLabel;

  useEffect(
    () => () => {
      if (resetTimerRef.current) clearTimeout(resetTimerRef.current);
    },
    [],
  );

  async function copy() {
    if (resetTimerRef.current) clearTimeout(resetTimerRef.current);
    try {
      await navigator.clipboard.writeText(value);
      setState("copied");
    } catch {
      setState("error");
    }
    resetTimerRef.current = setTimeout(
      () => setState("idle"),
      PROBLEM_UI_POLICY.COPY_FEEDBACK_DURATION_MS,
    );
  }

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size={showLabel ? "sm" : "icon"}
        className={cn(
          "cursor-pointer text-muted-foreground",
          state === "copied" && "text-status-success",
          state === "error" && "text-destructive",
          className,
        )}
        onClick={() => void copy()}
        aria-label={label}
        title={label}
      >
        {state === "copied" ? (
          <Check aria-hidden="true" />
        ) : state === "error" ? (
          <TriangleAlert aria-hidden="true" />
        ) : (
          <Copy aria-hidden="true" />
        )}
        {showLabel ? <span>{label}</span> : null}
      </Button>
      <span className="sr-only" role="status" aria-live="polite">
        {state === "idle" ? "" : label}
      </span>
    </>
  );
}
