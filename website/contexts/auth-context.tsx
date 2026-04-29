"use client";

/**
 * Global authentication context.
 * Uses JWT decode for instant auth state (no API call on refresh).
 * Caches user profile in localStorage for instant display.
 * Background revalidation via getProfile() keeps data fresh.
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
import { clearTokens, ApiError } from "@/lib/api-client";
import { decodeJwt, isTokenExpired } from "@/lib/jwt";
import { authService } from "@/services/auth.service";
import { userService } from "@/services/user.service";
import type { LoginRequest } from "@/types/auth";
import type { UserProfile } from "@/types/user";
import { createDefaultProfile } from "@/types/user";

const TOKEN_KEY = "judgify_access_token";
const PROFILE_CACHE_KEY = "judgify_user_profile";

/** Read cached profile from localStorage. */
function getCachedProfile(): UserProfile | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(PROFILE_CACHE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

/** Save profile to localStorage cache. */
function setCachedProfile(profile: UserProfile | null): void {
  if (profile) {
    localStorage.setItem(PROFILE_CACHE_KEY, JSON.stringify(profile));
  } else {
    localStorage.removeItem(PROFILE_CACHE_KEY);
  }
}

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

  // Instant auth check via JWT decode — no API call, no loading flash
  const [isAuthenticated, setIsAuthenticated] = useState(() => {
    if (typeof window === "undefined") return false;
    const token = localStorage.getItem(TOKEN_KEY);
    return !!token && !isTokenExpired(token);
  });

  // Load cached profile instantly from localStorage
  const [user, setUserState] = useState<UserProfile | null>(() => {
    if (typeof window === "undefined") return null;
    return getCachedProfile();
  });

  // No loading state needed — auth is determined synchronously from JWT
  const [isLoading] = useState(false);

  // Wrapper that syncs profile to both state and localStorage cache
  const setUser = useCallback((profile: UserProfile | null) => {
    setUserState(profile);
    setCachedProfile(profile);
  }, []);

  // Background revalidation: fetch fresh profile without blocking UI
  useEffect(() => {
    if (!isAuthenticated) return;

    userService.getProfile()
      .then((fresh) => setUser(fresh))
      .catch((err) => {
        if (err instanceof ApiError && err.code === 401) {
          clearTokens();
          setCachedProfile(null);
          setIsAuthenticated(false);
          setUserState(null);
        }
      });
  }, [isAuthenticated, setUser]);

  const login = useCallback(
    async (req: LoginRequest) => {
      const res = await authService.login(req);
      setIsAuthenticated(true);

      // Decode JWT for instant username display
      const payload = decodeJwt(res.access_token);
      if (payload) {
        setUser(createDefaultProfile(payload.username));
      }

      // Fetch full profile in background
      userService.getProfile()
        .then((full) => setUser(full))
        .catch(() => {});

      router.push("/arena");
    },
    [router, setUser]
  );

  const logout = useCallback(() => {
    clearTokens();
    setCachedProfile(null);
    setIsAuthenticated(false);
    setUserState(null);
    router.push("/arena");
  }, [router]);

  const value = useMemo<AuthContextValue>(
    () => ({ isAuthenticated, isLoading, user, login, logout, setUser }),
    [isAuthenticated, isLoading, user, login, logout, setUser]
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
