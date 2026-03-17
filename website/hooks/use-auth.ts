"use client";

/**
 * Custom hook for authentication state management.
 * Provides login status check and logout functionality.
 */

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { clearTokens } from "@/lib/api-client";

/** Manages authentication state by checking for stored access token. */
export function useAuth() {
  const router = useRouter();
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem("judgify_access_token");
    setIsAuthenticated(!!token);
    setIsLoading(false);
  }, []);

  /** Clears tokens and redirects to login page. */
  const logout = useCallback(() => {
    clearTokens();
    setIsAuthenticated(false);
    router.push("/login");
  }, [router]);

  return { isAuthenticated, isLoading, logout };
}
