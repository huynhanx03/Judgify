"use client";

/**
 * Global authentication context.
 * Provides auth state (isAuthenticated, user, isLoading) and actions (login, logout)
 * to the entire app. Checks localStorage token on mount.
 */

import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useMemo,
  type ReactNode,
} from "react";
import { useRouter } from "next/navigation";
import { clearTokens } from "@/lib/api-client";
import { login as loginService } from "@/services/auth.service";
import type { LoginRequest } from "@/types/auth";
import type { UserProfile } from "@/types/user";

const TOKEN_KEY = "judgify_access_token";

interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: UserProfile | null;
  login: (req: LoginRequest) => Promise<void>;
  logout: () => void;
  setUser: (user: UserProfile | null) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [user, setUser] = useState<UserProfile | null>(null);

  useEffect(() => {
    const token = localStorage.getItem(TOKEN_KEY);
    setIsAuthenticated(!!token);
    setIsLoading(false);
  }, []);

  const login = useCallback(
    async (req: LoginRequest) => {
      await loginService(req);
      setIsAuthenticated(true);
      router.push("/arena");
    },
    [router]
  );

  const logout = useCallback(() => {
    clearTokens();
    setIsAuthenticated(false);
    setUser(null);
    router.push("/arena");
  }, [router]);

  const value = useMemo<AuthContextValue>(
    () => ({ isAuthenticated, isLoading, user, login, logout, setUser }),
    [isAuthenticated, isLoading, user, login, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/** Hook to consume auth context. Must be used within AuthProvider. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
