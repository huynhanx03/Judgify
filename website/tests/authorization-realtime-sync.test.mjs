import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import ts from "typescript";

const ROOT = process.cwd();

async function loadSyncModule() {
  const source = await readFile(
    path.join(ROOT, "lib/auth/authorization-revision-sync.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  }).outputText;
  const loadedModule = { exports: {} };
  new Function("require", "module", "exports", compiled)(
    () => ({}),
    loadedModule,
    loadedModule.exports,
  );
  return loadedModule.exports;
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

async function flush() {
  await Promise.resolve();
  await Promise.resolve();
}

test("authorization event requires matching topic, revision, and timestamp", async () => {
  const { authorizationRevisionFromEvent } = await loadSyncModule();
  const topic = "authorization:capabilities";
  const data = { revision: 12, changed_at: "2026-07-22T09:00:00Z" };
  const envelope = {
    v: 1,
    op: "event",
    type: "authorization.capabilities.changed.v1",
    topic,
    aggregate_version: 12,
  };

  assert.equal(authorizationRevisionFromEvent(data, envelope, topic), 12);
  assert.equal(
    authorizationRevisionFromEvent(data, { ...envelope, aggregate_version: 11 }, topic),
    null,
  );
  assert.equal(
    authorizationRevisionFromEvent(data, { ...envelope, topic: "user:other" }, topic),
    null,
  );
  assert.equal(
    authorizationRevisionFromEvent({ ...data, changed_at: "invalid" }, envelope, topic),
    null,
  );
});

test("revision sync ignores stale events and coalesces a newer in-flight target", async () => {
  const { AuthorizationRevisionSync } = await loadSyncModule();
  let currentRevision = 5;
  let currentSession = true;
  const requests = [];
  const sync = new AuthorizationRevisionSync({
    currentRevision: () => currentRevision,
    isCurrentSession: () => currentSession,
    refresh: () => {
      const request = deferred();
      requests.push(request);
      return request.promise;
    },
    initialRetryDelayMs: 10,
    maximumRetryDelayMs: 100,
    retryMultiplier: 2,
  });

  sync.requestRevision(5);
  assert.equal(requests.length, 0);
  sync.requestRevision(6);
  sync.requestRevision(7);
  assert.equal(requests.length, 1);

  currentRevision = 6;
  requests[0].resolve("ready");
  await flush();
  assert.equal(requests.length, 2);

  currentRevision = 7;
  requests[1].resolve("ready");
  await flush();
  assert.equal(requests.length, 2);

  currentSession = false;
  sync.requestRevision(8);
  assert.equal(requests.length, 2);
  sync.stop();
});

test("revision sync retries a transient projection gap and stop fences timers", async () => {
  const { AuthorizationRevisionSync } = await loadSyncModule();
  let currentRevision = 20;
  let calls = 0;
  const sync = new AuthorizationRevisionSync({
    currentRevision: () => currentRevision,
    isCurrentSession: () => true,
    refresh: async () => {
      calls += 1;
      if (calls === 2) currentRevision = 21;
      return "ready";
    },
    initialRetryDelayMs: 1,
    maximumRetryDelayMs: 2,
    retryMultiplier: 2,
  });

  sync.requestRevision(21);
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.equal(calls, 2);

  const stopped = new AuthorizationRevisionSync({
    currentRevision: () => 1,
    isCurrentSession: () => true,
    refresh: async () => "error",
    initialRetryDelayMs: 5,
    maximumRetryDelayMs: 10,
    retryMultiplier: 2,
  });
  stopped.requestRevision(2);
  stopped.stop();
  await new Promise((resolve) => setTimeout(resolve, 20));
});

test("session projection is the only auth capability authority", async () => {
  const [authContext, sessionContext, protocol] = await Promise.all([
    readFile(path.join(ROOT, "contexts/auth-context.tsx"), "utf8"),
    readFile(path.join(ROOT, "contexts/session-context.tsx"), "utf8"),
    readFile(path.join(ROOT, "constants/realtime.ts"), "utf8"),
  ]);

  assert.match(protocol, /"authorization\.capabilities\.changed\.v1"/);
  assert.match(protocol, /authorizationCapabilitiesTopic/);
  assert.match(authContext, /createCapabilityIndexFromKeys/);
  assert.match(authContext, /authenticated\.authorization_revision/);
  assert.match(sessionContext, /sessionService\.meOrNull/);
  assert.doesNotMatch(
    authContext,
    /WebSocketClient|AuthorizationRevisionSync|sessionTransport/,
  );
});
