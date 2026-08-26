import assert from "node:assert/strict";
import { test } from "vitest";

import {
  RefreshCoordinator,
  REFRESH_COORDINATOR_STORAGE_KEY,
  type CoordinatorBus,
  type CoordinatorStorage,
} from "@/lib/auth/refresh-coordinator";
import { ApiError } from "@/lib/api/error";

class MemoryStorage implements CoordinatorStorage {
  private values = new Map<string, string>();

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

class MemoryBus implements CoordinatorBus {
  private listeners = new Set<(message: unknown) => void>();

  publish(message: unknown): void {
    for (const listener of this.listeners) listener(message);
  }

  subscribe(listener: (message: unknown) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}

test("concurrent callers in one tab share one refresh command", async () => {
  const storage = new MemoryStorage();
  const bus = new MemoryBus();
  let refreshCalls = 0;
  let authenticated = false;
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage,
    bus,
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  const rotate = async () => {
    refreshCalls += 1;
    authenticated = true;
  };
  const revalidate = async () => (authenticated ? { id: "session" } : null);

  const [first, second] = await Promise.all([
    coordinator.coordinate(rotate, revalidate),
    coordinator.coordinate(rotate, revalidate),
  ]);

  assert.equal(refreshCalls, 1);
  assert.deepEqual(first, { id: "session" });
  assert.deepEqual(second, { id: "session" });
});

test("unknown outcome preserves the same non-secret attempt UUID for recovery", async () => {
  const storage = new MemoryStorage();
  const bus = new MemoryBus();
  const attempts: string[] = [];
  let now = 1_000;
  let call = 0;
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage,
    bus,
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => now,
  });

  const rotate = async (attemptID: string) => {
    attempts.push(attemptID);
    call += 1;
    if (call === 1) throw new TypeError("response lost");
  };

  await assert.rejects(
    () => coordinator.coordinate(rotate, async () => null),
    /response lost/,
  );
  now += 1_000;
  const recovered = await coordinator.coordinate(rotate, async () => ({
    id: "session",
  }));

  assert.deepEqual(attempts, [
    "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
  ]);
  assert.deepEqual(recovered, { id: "session" });
});

test("persisted coordinator record is bounded and contains no credential data", async () => {
  const storage = new MemoryStorage();
  const bus = new MemoryBus();
  const coordinator = new RefreshCoordinator<never>({
    storage,
    bus,
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  await assert.rejects(
    () =>
      coordinator.coordinate(
        async () => {
          throw new TypeError("offline");
        },
        async () => null,
      ),
    /offline/,
  );

  const raw = storage.getItem("judgify:session-refresh:v1");
  assert.ok(raw);
  assert.deepEqual(Object.keys(JSON.parse(raw)).sort(), [
    "attempt_id",
    "expires_at",
    "lease_owner",
  ]);
  assert.doesNotMatch(raw, /token|cookie|csrf|user|profile|session_id/i);
});

test("forced refresh rotates an authenticated session after step-up", async () => {
  const storage = new MemoryStorage();
  let refreshCalls = 0;
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage,
    bus: new MemoryBus(),
    locks: {
      async request(_name, _options, callback) {
        return callback();
      },
    },
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  const session = await coordinator.coordinate(
    async () => {
      refreshCalls += 1;
    },
    async () => ({ id: "session" }),
    { forceRotation: true },
  );

  assert.equal(refreshCalls, 1);
  assert.deepEqual(session, { id: "session" });
});

test("concurrent forced callers share one credential rotation", async () => {
  let refreshCalls = 0;
  let releaseRotation: (() => void) | undefined;
  const rotationGate = new Promise<void>((resolve) => {
    releaseRotation = resolve;
  });
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage: new MemoryStorage(),
    bus: new MemoryBus(),
    locks: {
      async request(_name, _options, callback) {
        return callback();
      },
    },
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });
  const rotate = async () => {
    refreshCalls += 1;
    await rotationGate;
  };
  const revalidate = async () => ({ id: "session" });

  const first = coordinator.coordinate(rotate, revalidate, {
    forceRotation: true,
  });
  const second = coordinator.coordinate(rotate, revalidate, {
    forceRotation: true,
  });
  releaseRotation?.();

  assert.deepEqual(await first, { id: "session" });
  assert.deepEqual(await second, { id: "session" });
  assert.equal(refreshCalls, 1);
});

test("a lock holder reuses an already authenticated session without rotating", async () => {
  let rotations = 0;
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage: new MemoryStorage(),
    bus: new MemoryBus(),
    locks: { async request(_name, _options, callback) { return callback(); } },
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  const session = await coordinator.coordinate(
    async () => { rotations += 1; },
    async () => ({ id: "current" }),
  );

  assert.deepEqual(session, { id: "current" });
  assert.equal(rotations, 0);
});

test("an invalid refresh credential settles to anonymous and clears its receipt", async () => {
  const storage = new MemoryStorage();
  const published: unknown[] = [];
  const coordinator = new RefreshCoordinator<never>({
    storage,
    bus: { publish: (message) => published.push(message), subscribe: () => () => undefined },
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  const result = await coordinator.coordinate(
    async () => {
      throw new ApiError({
        code: "session.unauthorized",
        status: 401,
        cid: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313d",
      });
    },
    async () => null,
  );

  assert.equal(result, null);
  assert.equal(storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY), null);
  assert.match(JSON.stringify(published), /anonymous/);
});

test("a successful rotation with no session settles anonymous rather than retrying", async () => {
  const published: unknown[] = [];
  const coordinator = new RefreshCoordinator<never>({
    storage: new MemoryStorage(),
    bus: { publish: (message) => published.push(message), subscribe: () => () => undefined },
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => 1_000,
  });

  assert.equal(await coordinator.coordinate(async () => undefined, async () => null), null);
  assert.match(JSON.stringify(published), /anonymous/);
});

test("a follower accepts the leader's authenticated settlement before taking over", async () => {
  const storage = new MemoryStorage();
  const bus = new MemoryBus();
  let now = 1_000;
  storage.setItem(REFRESH_COORDINATOR_STORAGE_KEY, JSON.stringify({
    attempt_id: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313b",
    lease_owner: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313c",
    expires_at: 2_000,
  }));
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage,
    bus,
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => now,
    followerWaitMs: 100,
    sleep: async () => {
      bus.publish({ v: 1, attempt_id: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313b", generation: 0, outcome: "authenticated", occurred_at: now });
      now += 100;
    },
  });
  let rotations = 0;

  const session = await coordinator.coordinate(
    async () => { rotations += 1; },
    async () => ({ id: "leader-session" }),
  );

  assert.deepEqual(session, { id: "leader-session" });
  assert.equal(rotations, 0);
});

test("a follower takes over an unsettled receipt using its original attempt id", async () => {
  const storage = new MemoryStorage();
  let now = 1_000;
  const attemptID = "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313b";
  storage.setItem(REFRESH_COORDINATOR_STORAGE_KEY, JSON.stringify({
    attempt_id: attemptID,
    lease_owner: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313c",
    expires_at: 2_000,
  }));
  const coordinator = new RefreshCoordinator<{ id: string }>({
    storage,
    bus: new MemoryBus(),
    randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
    now: () => now,
    followerWaitMs: 100,
    sleep: async () => { now += 100; },
  });
  const attempts: string[] = [];
  let revalidations = 0;

  const session = await coordinator.coordinate(
    async (id) => { attempts.push(id); },
    async () => {
      revalidations += 1;
      return revalidations === 1 ? null : { id: "recovered" };
    },
  );

  assert.deepEqual(session, { id: "recovered" });
  assert.deepEqual(attempts, [attemptID]);
});

test("malformed, expired, and unsafe persisted receipts are discarded before rotation", async () => {
  const invalidReceipts = [
    "{",
    "[]",
    JSON.stringify({ attempt_id: "x" }),
    JSON.stringify({
      attempt_id: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313b",
      lease_owner: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313c",
      expires_at: 1_000,
    }),
    JSON.stringify({
      attempt_id: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313b",
      lease_owner: "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313c",
      expires_at: 99_999_999,
    }),
    "x".repeat(513),
  ];

  for (const raw of invalidReceipts) {
    const storage = new MemoryStorage();
    storage.setItem(REFRESH_COORDINATOR_STORAGE_KEY, raw);
    const coordinator = new RefreshCoordinator<{ id: string }>({
      storage,
      bus: new MemoryBus(),
      randomUUID: () => "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a",
      now: () => 1_000,
    });
    const result = await coordinator.coordinate(
      async () => undefined,
      async () => ({ id: "fresh" }),
    );
    assert.deepEqual(result, { id: "fresh" });
    assert.equal(storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY), null);
  }
});

test("configuration rejects receipt and follower timing outside bounded limits", () => {
  assert.throws(
    () => new RefreshCoordinator({ receiptTTLms: 999 }),
    /refresh receipt TTL/,
  );
  assert.throws(
    () => new RefreshCoordinator({ receiptTTLms: 3_600_001 }),
    /refresh receipt TTL/,
  );
  assert.throws(
    () => new RefreshCoordinator({ followerWaitMs: 49 }),
    /refresh follower wait/,
  );
  assert.throws(
    () => new RefreshCoordinator({ followerWaitMs: 30_001 }),
    /refresh follower wait/,
  );
});
