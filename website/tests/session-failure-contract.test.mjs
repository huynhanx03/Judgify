import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("one cookie transport owns unauthorized recovery without token state", async () => {
  const client = await source("lib/api/client.ts");

  assert.match(client, /credentials:\s*["']include["']/);
  assert.match(client, /setUnauthorizedHandler/);
  assert.match(client, /shouldRetryAfterRefresh/);
  assert.match(client, /SAFE_METHODS\.has\(method\)\s*\|\|\s*Boolean\(idempotencyKey\)/);
  assert.match(client, /requestOptions\.retryUnauthorized\s*!==\s*false/);
  assert.doesNotMatch(
    client,
    /headers\.set\(\s*["']Authorization|Bearer\s|access_token|refresh_token/,
  );
  assert.doesNotMatch(client, /localStorage|sessionStorage/);
});

test("canonical session provider keeps transient failure distinct from anonymous", async () => {
  const [session, auth] = await Promise.all([
    source("contexts/session-context.tsx"),
    source("contexts/auth-context.tsx"),
  ]);

  assert.match(session, /status:\s*["']anonymous["']/);
  assert.match(session, /status:\s*["']authenticated["']/);
  assert.match(session, /status:\s*["']error["']/);
  assert.match(session, /error\.status\s*===\s*401/);
  assert.match(session, /sessionService\.meOrNull/);
  assert.match(session, /coordinator\.coordinate/);
  assert.match(session, /apiTransport\.setUnauthorizedHandler/);
  assert.doesNotMatch(session + auth, /decodeJwt|sessionTransport|acceptAccessToken/);
});

test("unknown logout outcome converges only through the authoritative session read", async () => {
  const session = await source("contexts/session-context.tsx");
  const logoutStart = session.indexOf("const logout = useCallback");
  const hasStart = session.indexOf("const has = useCallback", logoutStart);
  assert.notEqual(logoutStart, -1);
  assert.notEqual(hasStart, -1);
  const logout = session.slice(logoutStart, hasStart);

  assert.match(logout, /authService\.logout\(attempt\.attempt_id\)/);
  assert.match(logout, /sessionService\.meOrNull/);
  assert.match(logout, /canonical\s*===\s*null/);
  assert.match(logout, /commandAttemptStore\.resolve/);
  assert.doesNotMatch(logout, /localStorage|token|clearLocalSession/);
});
