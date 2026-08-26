import { AUTH_API } from "@/constants/api/auth";
import { api } from "@/lib/api/client";
import { entityIDSchema } from "@/lib/api/contracts";
import { ApiError } from "@/lib/api/error";
import { commandAttemptStore } from "@/lib/api/idempotency";
import {
  authSessionListSchema,
  sessionMutationSchema,
  sessionViewSchema,
} from "@/lib/auth/session-schema";
import type {
  AuthSessionListQuery,
  AuthSessionListResponse,
  AuthSessionMutationResponse,
  SessionView,
} from "@/types/session";

const DEFAULT_SESSION_PAGE_SIZE = 20;
const MAX_SESSION_PAGE_SIZE = 50;
const MAX_SESSION_CURSOR_LENGTH = 128;

function normalizeLimit(limit: number | undefined): number {
  if (limit === undefined) return DEFAULT_SESSION_PAGE_SIZE;
  if (!Number.isInteger(limit) || limit < 1 || limit > MAX_SESSION_PAGE_SIZE) {
    throw new RangeError("session page size is outside the supported boundary");
  }
  return limit;
}

function sessionListEndpoint(query: AuthSessionListQuery = {}): string {
  const params = new URLSearchParams();
  params.set("limit", String(normalizeLimit(query.limit)));
  const cursor = query.cursor?.trim();
  if (cursor) {
    if (cursor.length > MAX_SESSION_CURSOR_LENGTH) {
      throw new RangeError("session cursor is outside the supported boundary");
    }
    params.set("cursor", cursor);
  }
  return `${AUTH_API.SESSIONS}?${params.toString()}`;
}

function currentSession(): Promise<SessionView> {
  return api<SessionView, never>(AUTH_API.SESSION_ME, {
    method: "GET",
    retryUnauthorized: false,
    schema: sessionViewSchema,
  });
}

async function recoverableSessionMutation(
  purpose: string,
  command: (idempotencyKey: string) => Promise<AuthSessionMutationResponse>,
): Promise<AuthSessionMutationResponse> {
  const attempt = commandAttemptStore.getOrCreate(purpose);
  try {
    const response = await command(attempt.attempt_id);
    commandAttemptStore.resolve(purpose, attempt.attempt_id);
    return response;
  } catch (error) {
    if (
      error instanceof ApiError &&
      !error.retryable &&
      error.status < 500
    ) {
      commandAttemptStore.resolve(purpose, attempt.attempt_id);
    }
    throw error;
  }
}

export const sessionService = {
  me(): Promise<SessionView> {
    return currentSession();
  },

  async meOrNull(): Promise<SessionView | null> {
    try {
      return await currentSession();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) return null;
      throw error;
    }
  },

  list(
    query: AuthSessionListQuery = {},
    signal?: AbortSignal,
  ): Promise<AuthSessionListResponse> {
    return api<AuthSessionListResponse, never>(sessionListEndpoint(query), {
      method: "GET",
      signal,
      schema: authSessionListSchema,
    });
  },

  revoke(id: string): Promise<AuthSessionMutationResponse> {
    const sessionID = entityIDSchema.parse(id.trim());
    return recoverableSessionMutation(
      `session-revoke-${sessionID}`,
      (idempotencyKey) =>
        api<AuthSessionMutationResponse, never>(
          AUTH_API.SESSION(sessionID),
          {
            method: "DELETE",
            idempotencyKey,
            schema: sessionMutationSchema,
          },
        ),
    );
  },

  logoutAll(): Promise<AuthSessionMutationResponse> {
    return recoverableSessionMutation(
      "session-logout-all",
      (idempotencyKey) =>
        api<AuthSessionMutationResponse, never>(
          AUTH_API.LOGOUT_ALL_SESSIONS,
          {
            method: "POST",
            idempotencyKey,
            schema: sessionMutationSchema,
          },
        ),
    );
  },
};

export const SESSION_PAGE_SIZE = DEFAULT_SESSION_PAGE_SIZE;
export const SESSION_PAGE_SIZE_MAX = MAX_SESSION_PAGE_SIZE;
