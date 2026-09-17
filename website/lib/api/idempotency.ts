import { entityIDSchema } from "@/lib/api/contracts";
import type { EntityID } from "@/types/api";

const COMMAND_STORAGE_PREFIX = "judgify:command:v1:";
const DEFAULT_RECEIPT_TTL_MS = 15 * 60 * 1000;
const MAXIMUM_PURPOSE_LENGTH = 192;
const PURPOSE_PATTERN = /^[a-z][a-z0-9-]*$/;

export interface CommandAttempt {
  readonly purpose: string;
  readonly attempt_id: EntityID;
  readonly expires_at: number;
}

export interface CommandAttemptStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export interface CommandAttemptStoreOptions {
  storage?: CommandAttemptStorage;
  now?: () => number;
  randomUUID?: () => string;
  receiptTTLms?: number;
}

class MemoryCommandStorage implements CommandAttemptStorage {
  private readonly records = new Map<string, string>();

  getItem(key: string): string | null {
    return this.records.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.records.set(key, value);
  }

  removeItem(key: string): void {
    this.records.delete(key);
  }
}

function browserStorage(): CommandAttemptStorage {
  if (typeof window === "undefined") return new MemoryCommandStorage();
  try {
    const probe = `${COMMAND_STORAGE_PREFIX}probe`;
    window.localStorage.setItem(probe, "1");
    window.localStorage.removeItem(probe);
    return window.localStorage;
  } catch {
    return new MemoryCommandStorage();
  }
}

function validPurpose(purpose: string): string {
  if (
    purpose.length < 1 ||
    purpose.length > MAXIMUM_PURPOSE_LENGTH ||
    !PURPOSE_PATTERN.test(purpose)
  ) {
    throw new TypeError("invalid idempotency purpose");
  }
  return purpose;
}

function storageKey(purpose: string): string {
  return `${COMMAND_STORAGE_PREFIX}${validPurpose(purpose)}`;
}

function parseAttempt(
  raw: string | null,
  expectedPurpose: string,
  now: number,
): CommandAttempt | null {
  if (!raw || raw.length > 512) return null;
  try {
    const value: unknown = JSON.parse(raw);
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const record = value as Record<string, unknown>;
    if (
      Object.keys(record).length !== 3 ||
      record.purpose !== expectedPurpose ||
      typeof record.attempt_id !== "string" ||
      typeof record.expires_at !== "number" ||
      !Number.isSafeInteger(record.expires_at) ||
      record.expires_at <= now
    ) {
      return null;
    }
    const attemptID = entityIDSchema.parse(record.attempt_id);
    return {
      purpose: expectedPurpose,
      attempt_id: attemptID,
      expires_at: record.expires_at,
    };
  } catch {
    return null;
  }
}

/**
 * Persists only a purpose, UUID, and receipt-aligned expiry. It never stores a
 * request body, subject, session projection, cookie, CSRF value, or credential.
 */
export class CommandAttemptStore {
  private readonly storage: CommandAttemptStorage;
  private readonly now: () => number;
  private readonly randomUUID: () => string;
  private readonly receiptTTLMs: number;

  constructor(options: CommandAttemptStoreOptions = {}) {
    this.storage = options.storage ?? browserStorage();
    this.now = options.now ?? Date.now;
    this.randomUUID = options.randomUUID ?? (() => crypto.randomUUID());
    this.receiptTTLMs = options.receiptTTLms ?? DEFAULT_RECEIPT_TTL_MS;
    if (
      !Number.isSafeInteger(this.receiptTTLMs) ||
      this.receiptTTLMs < 1_000 ||
      this.receiptTTLMs > 60 * 60 * 1000
    ) {
      throw new RangeError("idempotency receipt TTL is outside the boundary");
    }
  }

  getOrCreate(purpose: string): CommandAttempt {
    const normalizedPurpose = validPurpose(purpose);
    const key = storageKey(normalizedPurpose);
    const now = this.now();
    const current = parseAttempt(
      this.storage.getItem(key),
      normalizedPurpose,
      now,
    );
    if (current) return current;

    const attempt: CommandAttempt = {
      purpose: normalizedPurpose,
      attempt_id: entityIDSchema.parse(this.randomUUID()),
      expires_at: now + this.receiptTTLMs,
    };
    this.storage.setItem(key, JSON.stringify(attempt));
    return attempt;
  }

  resolve(purpose: string, attemptID?: string): void {
    const key = storageKey(purpose);
    if (attemptID) {
      const current = parseAttempt(
        this.storage.getItem(key),
        validPurpose(purpose),
        this.now(),
      );
      if (current?.attempt_id !== attemptID) return;
    }
    this.storage.removeItem(key);
  }
}

export function createIdempotencyKey(): EntityID {
  return entityIDSchema.parse(crypto.randomUUID());
}

export const commandAttemptStore = new CommandAttemptStore();
