"use client";

/**
 * Conditionally renders children based on authentication state.
 * Use this to gate features like "Submit", "View History" behind login.
 * Shows a fallback (e.g. login prompt) when user is not authenticated.
 */

import { useAuth } from "@/contexts/auth-context";
import type { ReactNode } from "react";

interface ProtectedContentProps {
  children: ReactNode;
  fallback?: ReactNode;
}

export function ProtectedContent({ children, fallback = null }: ProtectedContentProps) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) return null;
  if (!isAuthenticated) return <>{fallback}</>;
  return <>{children}</>;
}
