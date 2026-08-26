import assert from "node:assert/strict";
import { test } from "vitest";

import {
  createBrowserCsrfTokenSource,
  createStaticCsrfTokenSource,
  csrfTokenFromCookie,
} from "@/lib/api/csrf";

const TOKEN = "0123456789abcdef";

test("reads a single valid CSRF cookie and ignores unrelated segments", () => {
  assert.equal(
    csrfTokenFromCookie(`theme=dark; judgify_csrf=${TOKEN}; locale=vi`),
    TOKEN,
  );
  assert.equal(csrfTokenFromCookie("theme=dark; invalid"), null);
  assert.equal(csrfTokenFromCookie(`token=${TOKEN}`, "token"), TOKEN);
});

test("decodes URL-encoded CSRF values and rejects malformed cookie inputs", () => {
  assert.equal(csrfTokenFromCookie("judgify_csrf=0123456789abc%64ef"), TOKEN);
  assert.throws(() => csrfTokenFromCookie("judgify_csrf=%"), /encoding/);
  assert.throws(() => csrfTokenFromCookie("judgify_csrf=short"), /boundary/);
  assert.throws(
    () => csrfTokenFromCookie(`judgify_csrf=${TOKEN}; judgify_csrf=${TOKEN}`),
    /duplicate/,
  );
});

test("static and browser sources expose only boundary-valid token values", () => {
  assert.equal(createStaticCsrfTokenSource(TOKEN).read(), TOKEN);
  assert.equal(createStaticCsrfTokenSource(null).read(), null);
  assert.throws(() => createStaticCsrfTokenSource("short"), /boundary/);

  Object.defineProperty(document, "cookie", {
    configurable: true,
    value: `judgify_csrf=${TOKEN}`,
  });
  assert.equal(createBrowserCsrfTokenSource().read(), TOKEN);
});
