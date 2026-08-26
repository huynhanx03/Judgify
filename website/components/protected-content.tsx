"use client";

/**
 * Conditionally renders children based on authentication state.
 * Use this to gate features like "Submit", "View History" behind login.
 * Shows a fallback (e.g. login prompt) when user is not authenticated.
 */

import { useSession } from "@/contexts/session-context";
import type { ReactNode } from "react";
import { text } from "@/i18n/text";
import { adminText } from "@/i18n/admin-text";

interface ProtectedContentProps {
  children: ReactNode;
  fallback?: ReactNode;
  errorFallback?: ReactNode;
  forbiddenFallback?: ReactNode;
  capability?: string;
}

export function ProtectedContent({
  children,
  fallback = null,
  errorFallback,
  forbiddenFallback,
  capability,
}: ProtectedContentProps) {
  const { state, has } = useSession();

  if (state.status === "loading") {
    return (
      <span className="sr-only" role="status" aria-live="polite">
        {text("COMMON.LOADING")}
      </span>
    );
  }
  if (state.status === "error") {
    return errorFallback ? (
      <>{errorFallback}</>
    ) : (
      <span role="alert" className="sr-only">
        {text("COMMON.ERROR")}
      </span>
    );
  }
  if (state.status === "anonymous") return <>{fallback}</>;
  if (capability && !has(capability)) {
    return forbiddenFallback ? (
      <>{forbiddenFallback}</>
    ) : (
      <span role="alert" className="sr-only">
        {adminText("ACCESS.FORBIDDEN_TITLE")}
      </span>
    );
  }
  return <>{children}</>;
}
