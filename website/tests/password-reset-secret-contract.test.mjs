import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("password reset consumes only a fragment token and erases browser-visible secrets", async () => {
  const [page, helper, authLayout] = await Promise.all([
    source("modules/auth/reset-password-page.tsx"),
    source("lib/auth/one-time-token.ts"),
    source("app/(auth)/layout.tsx"),
  ]);

  assert.match(page, /consumeOneTimeTokenFragment/);
  assert.match(page, /tokenConsumedRef\.current/);
  assert.match(helper, /location\.hash/);
  assert.match(helper, /history\.replaceState/);
  assert.match(helper, /searchParams\.delete/);
  assert.match(page, /APP_ROUTES\.LOGIN/);
  assert.match(authLayout, /referrer:\s*"no-referrer"/);

  assert.doesNotMatch(page, /useSearchParams/);
  assert.doesNotMatch(page, /(?:localStorage|sessionStorage|document\.cookie)/);
  assert.doesNotMatch(page, /jpr\d|jcv\d/);
  assert.doesNotMatch(helper, /\.get\(["']token["']\).*window\.location\.search/);
});
