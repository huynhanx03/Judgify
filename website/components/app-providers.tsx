"use client";

/**
 * Client-side providers wrapper.
 * Combines all client providers (Auth, etc.) in one component
 * so the root layout stays a Server Component.
 */

import type { ReactNode } from "react";
import { AuthProvider } from "@/contexts/auth-context";

export function AppProviders({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
