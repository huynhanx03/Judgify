import type { RealtimeEnvelope } from "@/lib/realtime/websocket-client";

export type AuthorizationRefreshResult =
  | "ready"
  | "error"
  | "invalid"
  | "cancelled";

interface AuthorizationRevisionSyncOptions {
  currentRevision: () => number | null;
  isCurrentSession: () => boolean;
  refresh: () => Promise<AuthorizationRefreshResult>;
  initialRetryDelayMs: number;
  maximumRetryDelayMs: number;
  retryMultiplier: number;
}

/**
 * Validates the complete authorization invalidation boundary. A type match is
 * insufficient: the topic and aggregate version must agree with the payload
 * before it is allowed to trigger a protected REST request.
 */
export function authorizationRevisionFromEvent(
  data: unknown,
  envelope: RealtimeEnvelope<unknown>,
  expectedTopic: string,
): number | null {
  if (
    typeof data !== "object" ||
    data === null ||
    !("revision" in data) ||
    !Number.isSafeInteger(data.revision) ||
    (data.revision as number) <= 0 ||
    !("changed_at" in data) ||
    typeof data.changed_at !== "string" ||
    !Number.isFinite(Date.parse(data.changed_at)) ||
    envelope.topic !== expectedTopic ||
    envelope.aggregate_version !== data.revision
  ) {
    return null;
  }
  return data.revision as number;
}

/**
 * Coalesces revision invalidations for exactly one browser session generation.
 * A transient REST/runtime race retries with bounded backoff; a new higher
 * revision interrupts that delay. `stop` fences every timer and in-flight
 * completion when logout/login replaces the session.
 */
export class AuthorizationRevisionSync {
  private targetRevision: number | null = null;
  private forceRequested = false;
  private refreshing = false;
  private stopped = false;
  private retryAttempt = 0;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(private readonly options: AuthorizationRevisionSyncOptions) {
    if (
      options.initialRetryDelayMs <= 0 ||
      options.maximumRetryDelayMs < options.initialRetryDelayMs ||
      options.retryMultiplier < 1
    ) {
      throw new Error("invalid authorization revision sync policy");
    }
  }

  requestRevision(revision: number): void {
    if (
      this.stopped ||
      !Number.isSafeInteger(revision) ||
      revision <= 0 ||
      !this.options.isCurrentSession()
    ) {
      return;
    }
    const current = this.options.currentRevision();
    if (
      revision <= (current ?? 0) &&
      (this.targetRevision === null || revision <= this.targetRevision)
    ) {
      return;
    }
    this.targetRevision = Math.max(this.targetRevision ?? 0, revision);
    this.kick(true);
  }

  /** Force one REST snapshot after a socket (re)connect or resync signal. */
  requestResync(): void {
    if (this.stopped || !this.options.isCurrentSession()) return;
    this.forceRequested = true;
    this.kick(true);
  }

  stop(): void {
    this.stopped = true;
    this.targetRevision = null;
    this.forceRequested = false;
    if (this.retryTimer !== null) {
      clearTimeout(this.retryTimer);
      this.retryTimer = null;
    }
  }

  private kick(interruptRetry = false): void {
    if (this.stopped || this.refreshing || !this.options.isCurrentSession()) {
      return;
    }
    if (this.retryTimer !== null) {
      if (!interruptRetry) return;
      clearTimeout(this.retryTimer);
      this.retryTimer = null;
    }
    void this.runRefresh();
  }

  private async runRefresh(): Promise<void> {
    if (this.stopped || this.refreshing || !this.options.isCurrentSession()) {
      return;
    }
    const current = this.options.currentRevision() ?? 0;
    if (
      !this.forceRequested &&
      (this.targetRevision === null || this.targetRevision <= current)
    ) {
      this.targetRevision = null;
      return;
    }

    const targetAtStart = this.targetRevision ?? 0;
    const forcedAtStart = this.forceRequested;
    this.forceRequested = false;
    this.refreshing = true;
    let result: AuthorizationRefreshResult;
    try {
      result = await this.options.refresh();
    } catch {
      result = "error";
    } finally {
      this.refreshing = false;
    }

    if (this.stopped || !this.options.isCurrentSession()) return;
    if (result === "invalid") {
      this.stop();
      return;
    }
    if (result !== "ready") {
      if (forcedAtStart) this.forceRequested = true;
      this.scheduleRetry();
      return;
    }

    this.retryAttempt = 0;
    const applied = this.options.currentRevision() ?? 0;
    if (this.targetRevision !== null && applied >= this.targetRevision) {
      this.targetRevision = null;
    }
    if (this.forceRequested) {
      this.kick();
      return;
    }
    if (this.targetRevision !== null && this.targetRevision > applied) {
      if (this.targetRevision > targetAtStart) {
        this.kick();
      } else {
        this.scheduleRetry();
      }
    }
  }

  private scheduleRetry(): void {
    if (
      this.stopped ||
      this.retryTimer !== null ||
      !this.options.isCurrentSession()
    ) {
      return;
    }
    const delay = Math.min(
      this.options.initialRetryDelayMs *
        this.options.retryMultiplier ** this.retryAttempt,
      this.options.maximumRetryDelayMs,
    );
    this.retryAttempt += 1;
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null;
      this.kick();
    }, delay);
  }
}
