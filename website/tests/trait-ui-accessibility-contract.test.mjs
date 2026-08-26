import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

const ROOT = process.cwd();

async function source(relativePath) {
  return readFile(path.join(ROOT, relativePath), "utf8");
}

test("interactive trait cards are keyboard buttons and honor reduced motion", async () => {
  const card = await source("modules/cultivation/trait-card.tsx");

  assert.match(card, /m\.button/);
  assert.match(card, /type="button"/);
  assert.match(card, /aria-pressed=\{selected\}/);
  assert.match(card, /focus-visible:ring/);
  assert.match(card, /useReducedMotion/);
  assert.doesNotMatch(card, /border-white\/5|text-emerald-400"/);
});

test("trait codex uses the shared accessible dialog primitive", async () => {
  const codex = await source(
    "modules/cultivation/trait-codex-modal.tsx",
  );

  assert.match(codex, /<Dialog\s+[\s\S]*?open=\{open\}/);
  assert.match(codex, /<DialogTitle/);
  assert.match(codex, /<DialogDescription/);
  assert.doesNotMatch(codex, /fixed inset-0|onClick=\{onClose\}/);
  assert.match(codex, /CODEX_ROOT_EMPTY/);
  assert.match(codex, /CODEX_TALENT_EMPTY/);
});
