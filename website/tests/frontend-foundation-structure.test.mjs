import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

const websiteRoot = new URL("../", import.meta.url);

async function source(relativePath) {
  return readFile(new URL(relativePath, websiteRoot), "utf8");
}

async function exists(relativePath) {
  try {
    await access(new URL(relativePath, websiteRoot));
    return true;
  } catch {
    return false;
  }
}

test("theme selector avoids effect-driven mounted state", async () => {
  const themeToggle = await source("components/theme-toggle.tsx");

  assert.match(themeToggle, /useSyncExternalStore/);
  assert.doesNotMatch(themeToggle, /useEffect|setMounted/);
  assert.match(themeToggle, /light/);
  assert.match(themeToggle, /dark/);
  assert.match(themeToggle, /system/);
});

test("retired SSE and ranking placeholder modules stay removed", async () => {
  assert.equal(await exists("hooks/use-contest-sse.ts"), false);
  assert.equal(
    await exists("modules/ranking/ranking-spiritual-root-grid.tsx"),
    false,
  );
});

test("authentication layout has no injected keyframes or perpetual decoration", async () => {
  const authLayout = await source("app/(auth)/layout.tsx");

  assert.doesNotMatch(authLayout, /dangerouslySetInnerHTML/);
  assert.doesNotMatch(authLayout, /infinite|duration-1000/);
});

test("common surfaces resolve copy through the typed catalog boundary", async () => {
  const commonSurfaces = [
    "components/confirm-dialog.tsx",
    "components/loading-spinner.tsx",
    "components/pagination-controls.tsx",
    "components/protected-content.tsx",
    "components/theme-toggle.tsx",
    "modules/layout/header.tsx",
    "constants/navigation.ts",
    "constants/admin-navigation.ts",
  ];

  for (const relativePath of commonSurfaces) {
    const contents = await source(relativePath);
    assert.match(
      contents,
      /@\/(?:i18n|constants)\/(?:admin-)?text/,
      `${relativePath} must use centralized copy`,
    );
    assert.doesNotMatch(
      contents,
      /dangerouslySetInnerHTML/,
      `${relativePath} must render catalog values as text`,
    );
  }

  const [typedCopyBoundary, compatibilityBoundary] = await Promise.all([
    source("i18n/text.ts"),
    source("constants/text.ts"),
  ]);
  assert.match(typedCopyBoundary, /import \{ vi \} from "@\/i18n\/catalog\.vi"/);
  assert.match(
    compatibilityBoundary,
    /export \{ vi as TEXT \} from "@\/i18n\/catalog\.vi"/,
  );
});
