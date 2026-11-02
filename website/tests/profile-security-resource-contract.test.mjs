import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("OAuth identities use the shared abortable resource state machine", async () => {
  const [card, service] = await Promise.all([
    source("modules/profile/oauth-accounts-card.tsx"),
    source("services/auth.service.ts"),
  ]);

  assert.match(
    card,
    /useRetryableResource<OAuthIdentityListResponse>/,
  );
  assert.match(card, /identitiesResource\.retry/);
  assert.doesNotMatch(card, /reloadVersion|let active|useEffect\(/);
  assert.match(
    service,
    /listOAuthIdentities\([\s\S]*signal\?: AbortSignal/,
  );
});

test("session pagination aborts obsolete requests and fences stale pages", async () => {
  const [card, service] = await Promise.all([
    source("modules/profile/session-security-card.tsx"),
    source("services/session.service.ts"),
  ]);

  assert.match(
    card,
    /useRetryableResource<AuthSessionListResponse>/,
  );
  assert.match(card, /sessionsResource\.retry/);
  assert.doesNotMatch(card, /reloadVersion|let active|useEffect\(/);
  assert.match(
    service,
    /list\([\s\S]*signal\?: AbortSignal[\s\S]*signal,/,
  );
});
