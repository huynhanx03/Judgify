import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();
const UUID = "019f6c4c-4004-7d3f-bdfd-b3ee2fe7313a";

async function loadRefreshCoordinator() {
  const source = await readFile(
    path.join(ROOT, "lib/auth/refresh-coordinator.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loaded = { exports: {} };
  const require = (specifier) => {
    if (specifier === "@/lib/api/contracts") {
      return {
        entityIDSchema: {
          parse(value) {
            if (value !== UUID) throw new TypeError("invalid UUID");
            return value;
          },
        },
      };
    }
    if (specifier === "@/lib/api/error") {
      return {
        ApiError: class ApiError extends Error {
          constructor({ status }) {
            super("api error");
            this.status = status;
          }
        },
      };
    }
    throw new Error(`unexpected import: ${specifier}`);
  };
  new Function("require", "module", "exports", compiled)(
    require,
    loaded,
    loaded.exports,
  );
  return loaded.exports;
}

class MemoryStorage {
  values = new Map();

  getItem(key) {
    return this.values.get(key) ?? null;
  }

  setItem(key, value) {
    this.values.set(key, value);
  }

  removeItem(key) {
    this.values.delete(key);
  }
}

class MemoryBus {
  listeners = new Set();

  publish(message) {
    for (const listener of this.listeners) listener(message);
  }

  subscribe(listener) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}

test("one tab coalesces concurrent refresh callers without storing credentials", async () => {
  const { RefreshCoordinator, REFRESH_COORDINATOR_STORAGE_KEY } =
    await loadRefreshCoordinator();
  const storage = new MemoryStorage();
  let rotateCalls = 0;
  let authenticated = false;
  const coordinator = new RefreshCoordinator({
    storage,
    bus: new MemoryBus(),
    locks: null,
    now: () => 1_000,
    randomUUID: () => UUID,
  });
  const rotate = async () => {
    rotateCalls += 1;
    authenticated = true;
  };
  const revalidate = async () =>
    authenticated ? { id: "server-session" } : null;

  const [first, second] = await Promise.all([
    coordinator.coordinate(rotate, revalidate),
    coordinator.coordinate(rotate, revalidate),
  ]);

  assert.equal(rotateCalls, 1);
  assert.deepEqual(first, { id: "server-session" });
  assert.deepEqual(second, first);
  assert.equal(storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY), null);
});

test("an unknown refresh outcome reuses the same bounded non-secret UUID", async () => {
  const { RefreshCoordinator, REFRESH_COORDINATOR_STORAGE_KEY } =
    await loadRefreshCoordinator();
  const storage = new MemoryStorage();
  const attempts = [];
  let now = 1_000;
  let fail = true;
  const coordinator = new RefreshCoordinator({
    storage,
    bus: new MemoryBus(),
    locks: null,
    now: () => now,
    randomUUID: () => UUID,
  });
  const rotate = async (attemptID) => {
    attempts.push(attemptID);
    if (fail) throw new TypeError("response lost");
  };

  await assert.rejects(
    () => coordinator.coordinate(rotate, async () => null),
    /response lost/,
  );
  const persisted = storage.getItem(REFRESH_COORDINATOR_STORAGE_KEY);
  assert.ok(persisted);
  assert.deepEqual(Object.keys(JSON.parse(persisted)).sort(), [
    "attempt_id",
    "expires_at",
    "lease_owner",
  ]);
  assert.doesNotMatch(persisted, /token|cookie|csrf|profile|password/i);

  fail = false;
  now += 100;
  const recovered = await coordinator.coordinate(
    rotate,
    async () => ({ id: "server-session" }),
  );
  assert.deepEqual(recovered, { id: "server-session" });
  assert.deepEqual(attempts, [UUID, UUID]);
});
