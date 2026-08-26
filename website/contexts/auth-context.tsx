"use client";

/**
 * Product authentication and authorization facade.
 *
 * Session authority lives exclusively in `session-context.tsx`; this facade
 * adds navigation and capability helpers without creating another credential
 * store, refresh loop, capability source, or WebSocket.
 */

import {
  useCallback,
  useMemo,
} from "react";
import { useRouter } from "next/navigation";
import {
  AUTH_BOOTSTRAP_STATUS,
  CAPABILITY_STATUS,
} from "@/constants/authorization";
import { APP_ROUTES } from "@/constants/routes";
import { useSession } from "@/contexts/session-context";
import { ApiError } from "@/lib/api/error";
import {
  createCapabilityIndexFromKeys,
  EMPTY_CAPABILITY_INDEX,
  hasAnyCapability as indexHasAnyCapability,
  hasAnyRequirement,
  hasCapability,
  type CapabilityIndex,
} from "@/lib/auth/capabilities";
import { authService } from "@/services/auth.service";
import { safeAppDestination } from "@/lib/auth/safe-navigation";
import type {
  BootstrapStatus,
  CapabilityRequirement,
  CapabilityStatus,
  LoginRequest,
  OAuthFinalizeLoginRequest,
  RegisterRequest,
} from "@/types/auth";
import type { SessionPrincipal } from "@/types/session";

interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  bootstrapStatus: BootstrapStatus;
  capabilityStatus: CapabilityStatus;
  capabilityIndex: CapabilityIndex;
  authorizationRevision: number | null;
  hasAnyCapability: boolean;
  user: SessionPrincipal | null;
  can: (resource: string, action: string) => boolean;
  canAny: (requirements: readonly CapabilityRequirement[]) => boolean;
  refreshCapabilities: () => Promise<boolean>;
  retryBootstrap: () => void;
  login: (request: LoginRequest, destination?: string) => Promise<void>;
  completeOAuth: (
    request?: OAuthFinalizeLoginRequest,
    destination?: string,
  ) => Promise<void>;
  register: (request: RegisterRequest, destination?: string) => Promise<void>;
  logout: (destination?: string) => Promise<void>;
  setUser: (user: SessionPrincipal | null) => void;
}

export function isAuthenticationInvalid(error: unknown): error is ApiError {
  return error instanceof ApiError && error.status === 401;
}

export function useAuth(): AuthContextValue {
  const router = useRouter();
  const session = useSession();
  const authenticated =
    session.state.status === "authenticated" ? session.state.session : null;
  const capabilityIndex = useMemo(
    () =>
      authenticated
        ? createCapabilityIndexFromKeys(
            authenticated.capabilities,
            authenticated.authorization_revision,
          )
        : EMPTY_CAPABILITY_INDEX,
    [authenticated],
  );

  const login = useCallback(
    async (
      request: LoginRequest,
      destination: string = APP_ROUTES.ARENA,
    ) => {
      await session.establish("password-login", (idempotencyKey) =>
        authService.login(request, idempotencyKey),
      );
      router.push(safeAppDestination(destination, APP_ROUTES.ARENA));
    },
    [router, session],
  );

  const register = useCallback(
    async (
      request: RegisterRequest,
      destination: string = APP_ROUTES.ARENA,
    ) => {
      await session.establish("password-registration", (idempotencyKey) =>
        authService.register(request, idempotencyKey),
      );
      router.push(safeAppDestination(destination, APP_ROUTES.ARENA));
    },
    [router, session],
  );

  const completeOAuth = useCallback(
    async (
      request: OAuthFinalizeLoginRequest = {},
      destination: string = APP_ROUTES.ARENA,
    ) => {
      await session.establish("oauth-finalize-login", (idempotencyKey) =>
        authService.finalizeOAuthLogin(request, idempotencyKey),
      );
      router.push(safeAppDestination(destination, APP_ROUTES.ARENA));
    },
    [router, session],
  );

  const logout = useCallback(
    async (destination: string = APP_ROUTES.ARENA) => {
      await session.logout();
      router.push(safeAppDestination(destination, APP_ROUTES.ARENA));
    },
    [router, session],
  );

  const retryBootstrap = useCallback(() => {
    void session.revalidate().catch(() => undefined);
  }, [session]);

  const refreshCapabilities = useCallback(async (): Promise<boolean> => {
    return (await session.revalidate()) !== null;
  }, [session]);

  const setUser = useCallback(
    (user: SessionPrincipal | null) => {
      if (user) session.updatePrincipal(user);
    },
    [session],
  );

  const can = useCallback(
    (resource: string, action: string) =>
      hasCapability(capabilityIndex, resource, action),
    [capabilityIndex],
  );

  const canAny = useCallback(
    (requirements: readonly CapabilityRequirement[]) =>
      hasAnyRequirement(capabilityIndex, requirements),
    [capabilityIndex],
  );

  const status = session.state.status;
  const capabilityStatus: CapabilityStatus =
    status === "loading"
      ? CAPABILITY_STATUS.LOADING
      : status === "error"
        ? CAPABILITY_STATUS.ERROR
        : status === "authenticated"
          ? CAPABILITY_STATUS.READY
          : CAPABILITY_STATUS.IDLE;
  const bootstrapStatus: BootstrapStatus =
    status === "loading"
      ? AUTH_BOOTSTRAP_STATUS.LOADING
      : status === "error"
        ? AUTH_BOOTSTRAP_STATUS.ERROR
        : AUTH_BOOTSTRAP_STATUS.READY;

  return {
    isAuthenticated: status === "authenticated",
    isLoading: status === "loading",
    bootstrapStatus,
    capabilityStatus,
    capabilityIndex,
    authorizationRevision: capabilityIndex.revision,
    hasAnyCapability: indexHasAnyCapability(capabilityIndex),
    user: authenticated?.user ?? null,
    can,
    canAny,
    refreshCapabilities,
    retryBootstrap,
    login,
    completeOAuth,
    register,
    logout,
    setUser,
  };
}
