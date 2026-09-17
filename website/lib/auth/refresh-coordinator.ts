import { entityIDSchema } from "@/lib/api/contracts";
import { ApiError } from "@/lib/api/error";

export const REFRESH_COORDINATOR_STORAGE_KEY =
  "judgify:session-refresh:v1";
const REFRESH_COORDINATOR_CHANNEL = "judgify:session-refresh:v1";
const REFRESH_COORDINATOR_LOCK = "judgify:session-refresh:v1";
const DEFAULT_RECEIPT_TTL_MS = 15 * 60 * 1000;
const DEFAULT_FOLLOWER_WAIT_MS = 4_000;

export interface RefreshCoordinatorRecord {
  attempt_id: string;
  lease_owner: string;
  expires_at: number;
}

interface RefreshCoordinatorMessage {
  v: 1;
  attempt_id: string;
  generation: number;
  outcome: "authenticated" | "anonymous" | "retryable";
  occurred_at: number;
}

export interface CoordinatorStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export interface CoordinatorBus {
  publish(message: unknown): void;
  subscribe(listener: (message: unknown) => void): () => void;
}

interface LockManagerLike {
  request<T>(
    name: string,
    options: { mode: "exclusive" },
    callback: () => Promise<T>,
  ): Promise<T>;
}

export interface RefreshCoordinatorOptions {
  storage?: CoordinatorStorage;
  bus?: CoordinatorBus;
  locks?: LockManagerLike | null;
  now?: () => number;
  randomUUID?: () => string;
  sleep?: (milliseconds: number) => Promise<void>;
  receiptTTLms?: number;
  followerWaitMs?: number;
  generation?: () => number;
}

export interface RefreshRequestOptions {
  /** Rotate even when the current access credential still authenticates. */
  forceRotation?: boolean;
}

class MemoryStorage implements CoordinatorStorage {
  private readonly values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }
}

class NoopBus implements CoordinatorBus {
  publish(): void {}
  subscribe(): () => void {
    return () => undefined;
  }
}

class BrowserBroadcastBus implements CoordinatorBus {
  private channel: BroadcastChannel | null = null;

  private getChannel(): BroadcastChannel {
    if (!this.channel) {
      this.channel = new BroadcastChannel(REFRESH_COORDINATOR_CHANNEL);
    }
    return this.channel;
  }

  publish(message: unknown): void {
    this.getChannel().postMessage(message);
  }

  subscribe(listener: (message: unknown) => void): () => void {
    const channel = this.getChannel();
    const handler = (event: MessageEvent<unknown>) => listener(event.data);
    channel.addEventListener("message", handler);
    return () => channel.removeEventListener("message", handler);
  }
}

function safeBrowserStorage(): CoordinatorStorage {
  if (typeof window === "undefined") return new MemoryStorage();
  try {
    const probe = `${REFRESH_COORDINATOR_STORAGE_KEY}:probe`;
    window.localStorage.setItem(probe, "1");
    window.localStorage.removeItem(probe);
    return window.localStorage;
  } catch {
    return new MemoryStorage();
  }
}

function safeBrowserBus(): CoordinatorBus {
  return typeof BroadcastChannel === "undefined"
    ? new NoopBus()
    : new BrowserBroadcastBus();
}

function safeBrowserLocks(): LockManagerLike | null {
  if (typeof navigator === "undefined") return null;
  return (
    (navigator as Navigator & { locks?: LockManagerLike }).locks ?? null
  );
}

function boundedMilliseconds(
  value: number,
  minimum: number,
  maximum: number,
  label: string,
): number {
  if (
    !Number.isSafeInteger(value) ||
    value < minimum ||
    value > maximum
  ) {
    throw new RangeError(`${label} is outside the supported boundary`);
  }
  return value;
}

function isCoordinatorMessage(
  value: unknown,
): value is RefreshCoordinatorMessage {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const record = value as Record<string, unknown>;
  return (
    Object.keys(record).length === 5 &&
    record.v === 1 &&
    typeof record.attempt_id === "string" &&
    typeof record.generation === "number" &&
    Number.isSafeInteger(record.generation) &&
    (record.outcome === "authenticated" ||
      record.outcome === "anonymous" ||
      record.outcome === "retryable") &&
    typeof record.occurred_at === "number" &&
    Number.isSafeInteger(record.occurred_at)
  );
}

/**
 * Coordinates exactly one refresh rotation per tab and, when Web Locks is
 * available, across tabs. Its localStorage fallback uses compare-and-verify
 * lease ownership plus bounded BroadcastChannel waiting.
 */
export class RefreshCoordinator<TSession> {
  private readonly storage: CoordinatorStorage;
  private readonly bus: CoordinatorBus;
  private readonly locks: LockManagerLike | null;
  private readonly now: () => number;
  private readonly randomUUID: () => string;
  private readonly sleep: (milliseconds: number) => Promise<void>;
  private readonly receiptTTLms: number;
  private readonly followerWaitMs: number;
  private readonly generation: () => number;
  private readonly ownerID: string;
  private inFlight: Promise<TSession | null> | null = null;
  private inFlightForcesRotation = false;

  constructor(options: RefreshCoordinatorOptions = {}) {
    this.storage = options.storage ?? safeBrowserStorage();
    this.bus = options.bus ?? safeBrowserBus();
    this.locks =
      options.locks === undefined ? safeBrowserLocks() : options.locks;
    this.now = options.now ?? Date.now;
    this.randomUUID = options.randomUUID ?? (() => crypto.randomUUID());
    this.sleep =
      options.sleep ??
      ((milliseconds) =>
        new Promise((resolve) => setTimeout(resolve, milliseconds)));
    this.receiptTTLms = boundedMilliseconds(
      options.receiptTTLms ?? DEFAULT_RECEIPT_TTL_MS,
      1_000,
      60 * 60 * 1000,
      "refresh receipt TTL",
    );
    this.followerWaitMs = boundedMilliseconds(
      options.followerWaitMs ?? DEFAULT_FOLLOWER_WAIT_MS,
      50,
      30_000,
      "refresh follower wait",
    );
    this.generation = options.generation ?? (() => 0);
    this.ownerID = entityIDSchema.parse(this.randomUUID());
  }

  coordinate(
    rotate: (attemptID: string) => Promise<void>,
    revalidate: () => Promise<TSession | null>,
    options: RefreshRequestOptions = {},
  ): Promise<TSession | null> {
    const forceRotation = options.forceRotation === true;
    if (this.inFlight) {
      return forceRotation && !this.inFlightForcesRotation
        ? this.inFlight.then(
            () => this.coordinate(rotate, revalidate, options),
            () => this.coordinate(rotate, revalidate, options),
          )
        : this.inFlight;
    }
    const pending = this.run(rotate, revalidate, forceRotation);
    const tracked = pending.finally(() => {
      if (this.inFlight === tracked) {
        this.inFlight = null;
        this.inFlightForcesRotation = false;
      }
    });
    this.inFlight = tracked;
    this.inFlightForcesRotation = forceRotation;
    return tracked;
  }

  private async run(
    rotate: (attemptID: string) => Promise<void>,
    revalidate: () => Promise<TSession | null>,
    forceRotation: boolean,
  ): Promise<TSession | null> {
    if (this.locks) {
      return this.locks.request(
        REFRESH_COORDINATOR_LOCK,
        { mode: "exclusive" },
        async () => {
          if (!forceRotation) {
            const alreadyCurrent = await revalidate();
            if (alreadyCurrent) {
              this.clearRecord();
              return alreadyCurrent;
            }
          }
          return this.performAsLeader(
            this.getOrCreateRecord(true),
            rotate,
            revalidate,
          );
        },
      );
    }

    let record = this.getOrCreateRecord(false);
    if (record.lease_owner !== this.ownerID) {
      await this.waitForSettlement(record.attempt_id);
      const current = await revalidate();
      if (current) {
        this.clearRecord(record.attempt_id);
        return current;
      }
      record = this.takeOver(record.attempt_id);
    }
    return this.performAsLeader(record, rotate, revalidate);
  }

  private parseRecord(raw: string | null): RefreshCoordinatorRecord | null {
    if (!raw || raw.length > 512) return null;
    try {
      const value: unknown = JSON.parse(raw);
      if (!value || typeof value !== "object" || Array.isArray(value)) {
        return null;
      }
      const record = value as Record<string, unknown>;
      if (
        Object.keys(record).length !== 3 ||
        typeof record.attempt_id !== "string" ||
        typeof record.lease_owner !== "string" ||
        typeof record.expires_at !== "number" ||
        !Number.isSafeInteger(record.expires_at) ||
        record.expires_at <= this.now() ||
        record.expires_at > this.now() + this.receiptTTLms
      ) {
        return null;
      }
      entityIDSchema.parse(record.attempt_id);
      entityIDSchema.parse(record.lease_owner);
      return {
        attempt_id: record.attempt_id,
        lease_owner: record.lease_owner,
        expires_at: record.expires_at,
      };
    } catch {
      return null;
    }
  }

  private getOrCreateRecord(forceOwnership: boolean): RefreshCoordinatorRecord {
    const current = this.parseRecord(
      this.storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY),
    );
    if (current) {
      if (!forceOwnership || current.lease_owner === this.ownerID) {
        return current;
      }
      return this.writeAndVerify({
        ...current,
        lease_owner: this.ownerID,
      });
    }
    return this.writeAndVerify({
      attempt_id: entityIDSchema.parse(this.randomUUID()),
      lease_owner: this.ownerID,
      expires_at: this.now() + this.receiptTTLms,
    });
  }

  private takeOver(attemptID: string): RefreshCoordinatorRecord {
    const current = this.parseRecord(
      this.storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY),
    );
    if (current?.attempt_id === attemptID) {
      return this.writeAndVerify({
        ...current,
        lease_owner: this.ownerID,
      });
    }
    return this.getOrCreateRecord(true);
  }

  private writeAndVerify(
    record: RefreshCoordinatorRecord,
  ): RefreshCoordinatorRecord {
    this.storage.setItem(
      REFRESH_COORDINATOR_STORAGE_KEY,
      JSON.stringify(record),
    );
    const verified = this.parseRecord(
      this.storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY),
    );
    if (
      !verified ||
      verified.attempt_id !== record.attempt_id ||
      verified.lease_owner !== record.lease_owner ||
      verified.expires_at !== record.expires_at
    ) {
      throw new TypeError("refresh coordinator lease verification failed");
    }
    return verified;
  }

  private async waitForSettlement(attemptID: string): Promise<void> {
    let settled = false;
    const unsubscribe = this.bus.subscribe((message) => {
      if (
        isCoordinatorMessage(message) &&
        message.attempt_id === attemptID
      ) {
        settled = true;
      }
    });
    try {
      const deadline = this.now() + this.followerWaitMs;
      while (!settled && this.now() < deadline) {
        await this.sleep(Math.min(100, Math.max(1, deadline - this.now())));
      }
    } finally {
      unsubscribe();
    }
  }

  private publish(
    attemptID: string,
    outcome: RefreshCoordinatorMessage["outcome"],
  ): void {
    this.bus.publish({
      v: 1,
      attempt_id: attemptID,
      generation: this.generation(),
      outcome,
      occurred_at: this.now(),
    } satisfies RefreshCoordinatorMessage);
  }

  private clearRecord(attemptID?: string): void {
    if (attemptID) {
      const current = this.parseRecord(
        this.storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY),
      );
      if (current && current.attempt_id !== attemptID) return;
    }
    this.storage.removeItem(REFRESH_COORDINATOR_STORAGE_KEY);
  }

  private async performAsLeader(
    record: RefreshCoordinatorRecord,
    rotate: (attemptID: string) => Promise<void>,
    revalidate: () => Promise<TSession | null>,
  ): Promise<TSession | null> {
    let rotationError: unknown;
    try {
      await rotate(record.attempt_id);
    } catch (error) {
      rotationError = error;
    }

    let current: TSession | null = null;
    try {
      current = await revalidate();
    } catch (revalidationError) {
      if (rotationError === undefined) rotationError = revalidationError;
    }

    if (current) {
      this.clearRecord(record.attempt_id);
      this.publish(record.attempt_id, "authenticated");
      return current;
    }
    if (
      rotationError instanceof ApiError &&
      (rotationError.status === 400 || rotationError.status === 401)
    ) {
      this.clearRecord(record.attempt_id);
      this.publish(record.attempt_id, "anonymous");
      return null;
    }
    if (rotationError !== undefined) {
      this.publish(record.attempt_id, "retryable");
      throw rotationError;
    }

    this.clearRecord(record.attempt_id);
    this.publish(record.attempt_id, "anonymous");
    return null;
  }
}
