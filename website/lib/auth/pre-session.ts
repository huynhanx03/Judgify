import { AUTH_API } from "@/constants/api";
import { api } from "@/lib/api/client";
import {
  commandAttemptStore,
  type CommandAttemptStore,
} from "@/lib/api/idempotency";
import {
  createBrowserCsrfTokenSource,
  type CsrfTokenSource,
} from "@/lib/api/csrf";

const PRE_SESSION_PURPOSE = "pre-session-bootstrap";

export interface PreSessionBootstrapOptions {
  csrf?: CsrfTokenSource;
  attempts?: CommandAttemptStore;
  bootstrap?: (idempotencyKey: string, signal?: AbortSignal) => Promise<void>;
}

/**
 * Reuses an existing valid signed pre-session cookie. A reload/remount never
 * rotates it merely because React asked twice.
 *
 * `/auth/pre-session` is the only isolated backend cutover seam: the final
 * identity HTTP adapter may change the constant without touching callers.
 */
export class PreSessionBootstrap {
  private readonly csrf: CsrfTokenSource;
  private readonly attempts: CommandAttemptStore;
  private readonly bootstrap: (
    idempotencyKey: string,
    signal?: AbortSignal,
  ) => Promise<void>;
  private inFlight: Promise<string> | null = null;

  constructor(options: PreSessionBootstrapOptions = {}) {
    this.csrf = options.csrf ?? createBrowserCsrfTokenSource();
    this.attempts = options.attempts ?? commandAttemptStore;
    this.bootstrap =
      options.bootstrap ??
      ((idempotencyKey, signal) =>
        api<void, never>(AUTH_API.PRE_SESSION, {
          method: "POST",
          auth: "none",
          csrf: "omit",
          idempotencyKey,
          retryUnauthorized: false,
          signal,
        }));
  }

  ensure(signal?: AbortSignal): Promise<string> {
    const existing = this.csrf.read();
    if (existing) return Promise.resolve(existing);
    if (this.inFlight) return this.inFlight;
    const attempt = this.attempts.getOrCreate(PRE_SESSION_PURPOSE);
    const pending = this.bootstrap(attempt.attempt_id, signal).then(() => {
      const token = this.csrf.read();
      if (!token) {
        throw new TypeError(
          "pre-session bootstrap completed without a readable CSRF cookie",
        );
      }
      this.attempts.resolve(PRE_SESSION_PURPOSE, attempt.attempt_id);
      return token;
    });
    this.inFlight = pending;
    return pending.finally(() => {
      if (this.inFlight === pending) this.inFlight = null;
    });
  }
}

export const preSessionBootstrap = new PreSessionBootstrap();
