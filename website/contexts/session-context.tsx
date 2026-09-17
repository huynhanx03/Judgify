"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { apiTransport } from "@/lib/api/client";
import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";
import {
  RefreshCoordinator,
  type RefreshRequestOptions,
} from "@/lib/auth/refresh-coordinator";
import { authService } from "@/services/auth.service";
import { sessionService } from "@/services/session.service";
import type {
  SessionPrincipal,
  SessionView,
} from "@/types/session";

const SESSION_REVALIDATION_INTERVAL_MS = 5 * 60 * 1000;

export type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "authenticated"; session: SessionView }
  | { status: "error"; error: ApiError };

export type SessionIssuanceCommand = (
  idempotencyKey: string,
) => Promise<unknown>;

export interface SessionContextValue {
  state: SessionState;
  refresh(options?: RefreshRequestOptions): Promise<SessionView | null>;
  revalidate(): Promise<SessionView | null>;
  establish(
    purpose: string,
    command: SessionIssuanceCommand,
  ): Promise<SessionView>;
  logout(): Promise<void>;
  has(capability: string): boolean;
  updatePrincipal(principal: SessionPrincipal): void;
}

const SessionContext = createContext<SessionContextValue | null>(null);

export function anonymousSessionState(): SessionState {
  return { status: "anonymous" };
}

export function authenticatedSessionState(
  session: SessionView,
): SessionState {
  return { status: "authenticated", session };
}

export function sessionStateAfterFailure(error: ApiError): SessionState {
  return error.status === 401
    ? anonymousSessionState()
    : { status: "error", error };
}

function clientSessionError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  return new ApiError({
    code: "session_client_error",
    status: 503,
    cid: crypto.randomUUID(),
    retryable: true,
    cause: error,
  });
}

function initialState(
  initialSession: SessionView | null | undefined,
): SessionState {
  if (initialSession === undefined) return { status: "loading" };
  return initialSession
    ? authenticatedSessionState(initialSession)
    : anonymousSessionState();
}

export function SessionProvider({
  children,
  initialSession,
}: {
  children: ReactNode;
  initialSession?: SessionView | null;
}) {
  const [state, setState] = useState<SessionState>(() =>
    initialState(initialSession),
  );
  const stateRef = useRef(state);
  const generationRef = useRef(0);
  const lastValidatedAtRef = useRef(0);
  const mountedRef = useRef(true);
  const coordinatorRef = useRef<RefreshCoordinator<SessionView> | null>(null);
  if (!coordinatorRef.current) {
    coordinatorRef.current = new RefreshCoordinator<SessionView>({
      generation: () => generationRef.current,
    });
  }

  const commitState = useCallback((next: SessionState) => {
    if (!mountedRef.current) return;
    stateRef.current = next;
    setState(next);
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const revalidate = useCallback(async (): Promise<SessionView | null> => {
    try {
      const session = await sessionService.meOrNull();
      lastValidatedAtRef.current = Date.now();
      generationRef.current += 1;
      commitState(
        session
          ? authenticatedSessionState(session)
          : anonymousSessionState(),
      );
      return session;
    } catch (error) {
      const normalized = clientSessionError(error);
      commitState(sessionStateAfterFailure(normalized));
      throw normalized;
    }
  }, [commitState]);

  const refresh = useCallback(async (
    options: RefreshRequestOptions = {},
  ): Promise<SessionView | null> => {
    const coordinator = coordinatorRef.current;
    if (!coordinator) return null;
    try {
      const session = await coordinator.coordinate(
        (attemptID) => authService.refresh(attemptID),
        () => sessionService.meOrNull(),
        options,
      );
      lastValidatedAtRef.current = Date.now();
      generationRef.current += 1;
      commitState(
        session
          ? authenticatedSessionState(session)
          : anonymousSessionState(),
      );
      return session;
    } catch (error) {
      const normalized = clientSessionError(error);
      commitState(sessionStateAfterFailure(normalized));
      throw normalized;
    }
  }, [commitState]);

  const establish = useCallback(
    async (
      purpose: string,
      command: SessionIssuanceCommand,
    ): Promise<SessionView> => {
      const attempt = commandAttemptStore.getOrCreate(purpose);
      try {
        await command(attempt.attempt_id);
        const session = await sessionService.me();
        commandAttemptStore.resolve(purpose, attempt.attempt_id);
        lastValidatedAtRef.current = Date.now();
        generationRef.current += 1;
        commitState(authenticatedSessionState(session));
        return session;
      } catch (error) {
        const normalized = clientSessionError(error);
        if (!normalized.retryable && normalized.status < 500) {
          commandAttemptStore.resolve(purpose, attempt.attempt_id);
        }
        commitState(sessionStateAfterFailure(normalized));
        throw normalized;
      }
    },
    [commitState],
  );

  const logout = useCallback(async (): Promise<void> => {
    const purpose = "session-logout";
    const attempt = commandAttemptStore.getOrCreate(purpose);
    try {
      await authService.logout(attempt.attempt_id);
      const session = await sessionService.meOrNull();
      if (session) {
        throw new ApiError({
          code: "logout_not_confirmed",
          status: 409,
          cid: crypto.randomUUID(),
          retryable: true,
        });
      }
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
      generationRef.current += 1;
      commitState(anonymousSessionState());
    } catch (error) {
      // A lost logout response is resolved only by the canonical session read.
      const canonical = await sessionService.meOrNull().catch(() => undefined);
      if (canonical === null) {
        commandAttemptStore.resolve(purpose, attempt.attempt_id);
        generationRef.current += 1;
        commitState(anonymousSessionState());
        return;
      }
      const normalized = clientSessionError(error);
      commitState(sessionStateAfterFailure(normalized));
      throw normalized;
    }
  }, [commitState]);

  const has = useCallback((capability: string): boolean => {
    const current = stateRef.current;
    return (
      current.status === "authenticated" &&
      current.session.capabilities.includes(capability)
    );
  }, []);

  const updatePrincipal = useCallback((principal: SessionPrincipal): void => {
    const current = stateRef.current;
    if (
      current.status !== "authenticated" ||
      current.session.user.id !== principal.id
    ) {
      return;
    }
    commitState(
      authenticatedSessionState({
        ...current.session,
        user: { ...current.session.user, ...principal },
      }),
    );
  }, [commitState]);

  useEffect(() => {
    if (initialSession !== undefined) {
      lastValidatedAtRef.current = Date.now();
      return;
    }
    void revalidate().catch(() => undefined);
  }, [initialSession, revalidate]);

  useEffect(() => {
    apiTransport.setUnauthorizedHandler(async () => Boolean(await refresh()));
    return () => apiTransport.setUnauthorizedHandler(null);
  }, [refresh]);

  useEffect(() => {
    const onVisibilityChange = () => {
      if (
        document.visibilityState === "visible" &&
        Date.now() - lastValidatedAtRef.current >=
          SESSION_REVALIDATION_INTERVAL_MS
      ) {
        void revalidate().catch(() => undefined);
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () =>
      document.removeEventListener("visibilitychange", onVisibilityChange);
  }, [revalidate]);

  const value = useMemo<SessionContextValue>(
    () => ({
      state,
      refresh,
      revalidate,
      establish,
      logout,
      has,
      updatePrincipal,
    }),
    [establish, has, logout, refresh, revalidate, state, updatePrincipal],
  );

  return (
    <SessionContext.Provider value={value}>
      {children}
    </SessionContext.Provider>
  );
}

export function useSession(): SessionContextValue {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("useSession must be used within a SessionProvider");
  }
  return context;
}
