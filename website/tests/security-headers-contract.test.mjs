import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const [config, verifyScript] = await Promise.all([
  readFile(path.resolve(process.cwd(), "next.config.ts"), "utf8"),
  readFile(path.resolve(process.cwd(), "../scripts/verify/frontend.sh"), "utf8"),
]);

test("the frontend sends a cache-friendly baseline CSP with restrictive document directives", () => {
  assert.match(config, /Content-Security-Policy/);
  assert.match(config, /default-src 'self'/);
  assert.match(config, /object-src 'none'/);
  assert.match(config, /base-uri 'self'/);
  assert.match(config, /form-action 'self'/);
  assert.match(config, /frame-ancestors 'none'/);
  assert.match(config, /block-all-mixed-content/);
  assert.match(config, /upgrade-insecure-requests/);
  assert.match(config, /isDevelopment \? " 'unsafe-eval'" : ""/);
  assert.doesNotMatch(config, /nonce-/);
});

test("the production build boots its standalone artifact and checks the emitted CSP", () => {
  assert.match(verifyScript, /verify_build\(\)[\s\S]*verify_runtime_headers/);
  assert.match(verifyScript, /node \.next\/standalone\/server\.js/);
  assert.match(verifyScript, /production CSP must not allow unsafe-eval/);
});
