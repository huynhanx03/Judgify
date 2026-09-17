import assert from "node:assert/strict";
import { test } from "vitest";

import {
  DEFAULT_THEME_PREFERENCE,
  SEMANTIC_COLOR_TOKENS,
  THEME_PREFERENCES,
  THEME_STORAGE_KEY,
  type SemanticColorToken,
} from "@/design/tokens";

test("theme preferences expose light, dark, and system with system as default", () => {
  assert.deepEqual(THEME_PREFERENCES, ["light", "dark", "system"]);
  assert.equal(DEFAULT_THEME_PREFERENCE, "system");
});

test("theme preference uses a dedicated non-sensitive storage key", () => {
  assert.equal(THEME_STORAGE_KEY, "judgify-theme");
  assert.doesNotMatch(THEME_STORAGE_KEY, /token|session|profile|user/i);
});

test("semantic contract includes state, focus, editor, and surface colors", () => {
  const required: readonly SemanticColorToken[] = [
    "background",
    "foreground",
    "surface",
    "surface-raised",
    "border",
    "focus-ring",
    "success",
    "warning",
    "danger",
    "code-background",
    "editor-gutter",
  ];

  for (const token of required) {
    assert.ok(SEMANTIC_COLOR_TOKENS.includes(token), `missing ${token}`);
  }
});
