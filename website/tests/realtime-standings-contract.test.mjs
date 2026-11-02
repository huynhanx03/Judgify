import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("public contest standings opens an anonymous v1 WebSocket", async () => {
  const [api, config, client, provider, hook] = await Promise.all([
    source("constants/api/realtime.ts"),
    source("constants/realtime.ts"),
    source("lib/realtime/websocket-client.ts"),
    source("contexts/realtime-context.tsx"),
    source("hooks/use-contest-standings.ts"),
  ]);

  assert.match(api, /WEBSOCKET:\s*"\/ws"/);
  assert.match(config, /EVENT_CONTEST_STANDINGS_CHANGED_V1/);
  assert.match(config, /resolveAnonymousRealtimeWebSocketUrl/);
  assert.match(client, /mode:\s*typeof REALTIME_CONNECTION_MODE\.ANONYMOUS/);
  assert.match(provider, /mode:\s*REALTIME_CONNECTION_MODE\.ANONYMOUS/);
  assert.match(provider, /resolveAnonymousRealtimeWebSocketUrl/);
  assert.match(hook, /useRealtime\(\)/);
  assert.doesNotMatch(hook, /new WebSocketClient|issueTicket|realtimeService/);
});

test("contest standings treats shared-socket events as REST invalidations", async () => {
  const [config, client, provider, hook] = await Promise.all([
    source("constants/realtime.ts"),
    source("lib/realtime/websocket-client.ts"),
    source("contexts/realtime-context.tsx"),
    source("hooks/use-contest-standings.ts"),
  ]);

  assert.match(config, /OPERATION_SUBSCRIBE/);
  assert.match(config, /OPERATION_UNSUBSCRIBE/);
  assert.match(client, /subscribeToTopic/);
  assert.match(client, /unsubscribeFromTopic/);
  assert.match(hook, /subscribeToTopic/);
  assert.match(hook, /EVENT_CONTEST_STANDINGS_CHANGED_V1/);
  assert.match(hook, /state === "open"/);
  assert.match(provider, /next === "open"[\s\S]*client\.resubscribe\(\)/);
  assert.match(hook, /onResyncRequired\(\(\) => void refresh\(\)\)/);
  assert.match(hook, /onTopicStateChange/);
  assert.match(hook, /REALTIME_TOPIC_STATE\.SUBSCRIBED/);
  assert.doesNotMatch(hook, /setInterval|new WebSocketClient/);
});

test("contest standings snapshots are scoped to their contest query key", async () => {
  const hook = await source("hooks/use-contest-standings.ts");

  assert.match(hook, /interface StandingsSnapshot/);
  assert.match(hook, /snapshot\.key === contestId/);
  assert.match(hook, /setSnapshot\(\{ key: contestId, value: nextSnapshot \}\)/);
  assert.match(hook, /refreshStatus\.key === contestId/);
});

test("contest standings keeps initial loading distinct from an authoritative empty snapshot", async () => {
  const [hook, page] = await Promise.all([
    source("hooks/use-contest-standings.ts"),
    source("modules/contest/contest-detail-page.tsx"),
  ]);

  assert.match(hook, /hasLoaded:\s*boolean/);
  assert.match(hook, /hasLoaded:\s*currentSnapshot !== null/);
  assert.match(page, /hasLoaded:\s*standingsHasLoaded/);
  assert.match(page, /!standingsHasLoaded && !standingsHasError/);
});
