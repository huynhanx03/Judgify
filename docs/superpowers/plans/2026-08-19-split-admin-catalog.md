# Split Admin Copy Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the ADMIN namespace (~129 KB / 61% of source) out of the shared Vietnamese copy catalog into its own module so public routes no longer pay the bundle cost of admin copy.

**Architecture:** Extract the ADMIN top-level key from `i18n/catalog.vi.ts` into `i18n/catalog.admin.vi.ts` with its own `export const admin = {...} as const`. Refactor `i18n/types.ts` to be generic over any catalog type so both `TextKey`/`text()` and a new `AdminTextKey`/`adminText()` share the same LeafPaths/PathValue machinery. Wire up a `constants/admin-text.ts` re-export and migrate all 69 consumer files from `TEXT.ADMIN.` to `ADMIN_TEXT.` and from `text("ADMIN...")` to `adminText("...")`.

**Tech Stack:** Next.js App Router, TypeScript (strict), Vitest for unit tests

**Spec:** The task description in the system prompt above (the PROBLEM / DO THIS section).

## Global Constraints

- Stack is FIXED: Next.js App Router + React + Tailwind v4 + shadcn/Base UI + Framer Motion — do not add or swap libraries.
- Do NOT edit `website/scripts/check-bundle.mjs`.
- Do NOT break SSR/hydration; no hydration mismatches; light/dark/system theme must keep working.
- All Vietnamese string content is unchanged — this is a pure module-boundary refactor.
- No comments added unless the file already carries explanatory comments and the comment states a non-obvious invariant.
- Do NOT git commit/stage/reset/checkout/clean.
- `npm run verify` must stay green: `check:architecture`, `eslint`, `check:routes`, `check:user-role-contract`, `test:contracts`, `vitest`, `tsc --noEmit`.
- Do NOT touch: `modules/layout/header.tsx`, `components/app-providers.tsx`, `components/app-toaster.tsx`, `components/ui/tooltip.tsx`, or `services/`.
- The architecture checker (`scripts/check-frontend-architecture.mjs`) scans `constants/` but NOT `i18n/` — new files in `i18n/` are safe; `constants/admin-text.ts` must only re-export, no string literals.

---

## File Map

| Action | File | Responsibility |
|---|---|---|
| Create | `i18n/catalog.admin.vi.ts` | Admin-only copy: the private helper constants + functions + `export const admin = { ...ADMIN block... } as const` |
| Modify | `i18n/catalog.vi.ts` | Remove the ADMIN key and its private helper constants/functions; remove the `import { APP_LOCALE }` if APP_LOCALE is no longer used |
| Modify | `i18n/types.ts` | Make `LeafPaths`/`PathValue`/`ParamsFor` generic over `C` (any catalog); keep `TextKey`/`ParamsFor`/`TextArguments` working via `typeof vi`; export new `AdminTextKey`/`AdminParamsFor`/`AdminTextArguments` types via `typeof admin` |
| Create | `i18n/admin-text.ts` | Admin-scoped `adminText(key, params?)` accessor mirroring `text()` |
| Create | `constants/admin-text.ts` | `export { admin as ADMIN_TEXT } from "@/i18n/catalog.admin.vi"` |
| Modify | `i18n/catalog.test.ts` | Add `admin` catalog import; validate both catalogs; migrate `text("ADMIN.NAV.OVERVIEW")` → `adminText("NAV.OVERVIEW")`  |
| Modify | 12 ADMIN-ONLY files | Replace `import { TEXT } from "@/constants/text"` with `import { ADMIN_TEXT } from "@/constants/admin-text"` and `TEXT.ADMIN.` → `ADMIN_TEXT.` |
| Modify | 57 BOTH files | Add `import { ADMIN_TEXT } from "@/constants/admin-text"` alongside existing `TEXT` import; replace `TEXT.ADMIN.` → `ADMIN_TEXT.` |
| Modify | 8 `text("ADMIN...")` call-site files | Replace `import { text }` with `import { adminText }` from `@/i18n/admin-text`; replace `text("ADMIN.X.Y")` → `adminText("X.Y")` |

---

### Task 1: Create `i18n/catalog.admin.vi.ts`

**Files:**
- Create: `i18n/catalog.admin.vi.ts`

**Interfaces:**
- Consumes: `APP_LOCALE` from `@/i18n/locale` (used by formatter functions inside ADMIN)
- Produces: `export const admin` — identical shape to `vi.ADMIN`, typed `as const`

- [ ] **Step 1: Extract lines 1–170 (helper constants/functions) and lines 1604–3942 (ADMIN key body) from `i18n/catalog.vi.ts` into the new file.**

  The new file must:
  - Start with the JSDoc comment from `catalog.vi.ts` line 1 (the canonical copy comment), adjusted to say "Administrator surface copy."
  - Import `{ APP_LOCALE }` from `"@/i18n/locale"` (it is only referenced inside the ADMIN block).
  - Paste all private `const ADMIN_PERMISSION_*` records and `const ADMIN_CANONICAL_*` records verbatim (lines 14–157 of the original).
  - Paste all private helper functions verbatim (`localizedPermissionFallback`, `localizedPermissionResourceLabel`, etc., lines 159–169 of the original).
  - Export the admin content as:
    ```typescript
    export const admin = {
      // paste exactly the body of `ADMIN: { ... }` from catalog.vi.ts (lines 1605–3941 contents)
    } as const;
    ```
  - The body is everything between `ADMIN: {` (line 1604, exclusive) and the matching closing `},` (line 3942/3943, exclusive).

- [ ] **Step 2: Verify the new file compiles in isolation.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Zero errors (the rest of the codebase still imports from `catalog.vi.ts` unchanged at this point).

---

### Task 2: Strip the ADMIN block from `i18n/catalog.vi.ts`

**Files:**
- Modify: `i18n/catalog.vi.ts`

**Interfaces:**
- Consumes: nothing new
- Produces: `export const vi` — same shape minus the `ADMIN` key; `APP_LOCALE` import removed if unused

- [ ] **Step 1: Remove the private helper constants, helper functions, and the `ADMIN` key from `catalog.vi.ts`.**

  Delete:
  - Lines 9 (import APP_LOCALE) — APP_LOCALE is no longer used outside the ADMIN block.
  - Lines 14–169 (all `const ADMIN_PERMISSION_*`, `const ADMIN_CANONICAL_*`, and their helper functions).
  - Lines 1603–3943 (`// Admin` comment through closing `},` of the ADMIN key).

  The file should go from 3995 lines to approximately 2350 lines. The `vi` export must not contain an `ADMIN` key.

- [ ] **Step 2: Run tsc to confirm the public catalog still type-checks.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: TypeScript errors only about `TEXT.ADMIN` (because consumer files still reference it) — no errors inside `catalog.vi.ts` itself.

---

### Task 3: Refactor `i18n/types.ts` to be generic

**Files:**
- Modify: `i18n/types.ts`

**Interfaces:**
- Consumes: `typeof vi` from `@/i18n/catalog.vi`, `typeof admin` from `@/i18n/catalog.admin.vi`
- Produces:
  - `LeafPaths<T>` — generic, unchanged signature
  - `PathValue<T, Path>` — generic, unchanged signature
  - `TextKey` — `LeafPaths<typeof vi>` (unchanged for consumers)
  - `ParamsFor<Key extends TextKey>` — unchanged for consumers
  - `TextArguments<Key extends TextKey>` — unchanged for consumers
  - `AdminTextKey` — `LeafPaths<typeof admin>`
  - `AdminParamsFor<Key extends AdminTextKey>`
  - `AdminTextArguments<Key extends AdminTextKey>`

- [ ] **Step 1: Make `ParamsFor` generic so it works for any catalog type.**

  Rewrite `i18n/types.ts` to:
  ```typescript
  import type { vi } from "@/i18n/catalog.vi";
  import type { admin } from "@/i18n/catalog.admin.vi";

  export type InterpolationValue = string | number | boolean | bigint;

  type Join<Prefix extends string, Key extends string> = Prefix extends ""
    ? Key
    : `${Prefix}.${Key}`;

  /** Dot-delimited paths that terminate at a string or formatter function. */
  export type LeafPaths<T, Prefix extends string = ""> = {
    [Key in keyof T & string]: T[Key] extends string
      ? Join<Prefix, Key>
      : T[Key] extends (...args: infer _Arguments) => unknown
        ? Join<Prefix, Key>
        : T[Key] extends object
          ? LeafPaths<T[Key], Join<Prefix, Key>>
          : never;
  }[keyof T & string];

  export type PathValue<T, Path extends string> = Path extends keyof T
    ? T[Path]
    : Path extends `${infer Head}.${infer Tail}`
      ? Head extends keyof T
        ? PathValue<T[Head], Tail>
        : never
      : never;

  type PlaceholderNames<Value extends string> =
    Value extends `${string}{${infer Name}}${infer Rest}`
      ? Name | PlaceholderNames<Rest>
      : never;

  type NamedParams<Value extends string> = [PlaceholderNames<Value>] extends [never]
    ? undefined
    : Record<PlaceholderNames<Value>, InterpolationValue>;

  type CatalogParamsFor<C, Key extends LeafPaths<C>> = PathValue<
    C,
    Key
  > extends (...args: infer Arguments) => unknown
    ? { args: Arguments }
    : PathValue<C, Key> extends string
      ? NamedParams<PathValue<C, Key>>
      : never;

  type CatalogTextArguments<C, Key extends LeafPaths<C>> =
    CatalogParamsFor<C, Key> extends undefined
      ? [params?: undefined]
      : [params: CatalogParamsFor<C, Key>];

  // Public catalog types (unchanged public API)
  export type TextKey = LeafPaths<typeof vi>;
  export type ParamsFor<Key extends TextKey> = CatalogParamsFor<typeof vi, Key>;
  export type TextArguments<Key extends TextKey> = CatalogTextArguments<typeof vi, Key>;

  // Admin catalog types
  export type AdminTextKey = LeafPaths<typeof admin>;
  export type AdminParamsFor<Key extends AdminTextKey> = CatalogParamsFor<typeof admin, Key>;
  export type AdminTextArguments<Key extends AdminTextKey> = CatalogTextArguments<typeof admin, Key>;
  ```

- [ ] **Step 2: Verify types compile.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Only errors about `TEXT.ADMIN` in consumer files still — no errors in `i18n/types.ts`.

---

### Task 4: Create `i18n/admin-text.ts` and `constants/admin-text.ts`

**Files:**
- Create: `i18n/admin-text.ts`
- Create: `constants/admin-text.ts`

**Interfaces:**
- Consumes: `admin` from `@/i18n/catalog.admin.vi`; `AdminTextKey`, `AdminTextArguments`, `AdminParamsFor` from `@/i18n/types`
- Produces:
  - `adminText(key: AdminTextKey, ...params): string` — same runtime logic as `text()` but walks `admin` instead of `vi`
  - `ADMIN_TEXT` — re-export of `admin` with the stable alias name

- [ ] **Step 1: Create `i18n/admin-text.ts`.**

  ```typescript
  import { admin } from "@/i18n/catalog.admin.vi";
  import type {
    AdminTextArguments,
    AdminTextKey,
    InterpolationValue,
  } from "@/i18n/types";

  export type { AdminParamsFor, AdminTextKey } from "@/i18n/types";

  const UNSAFE_PATH_SEGMENTS = new Set(["__proto__", "constructor", "prototype"]);

  function resolveLeaf(key: string): unknown {
    let current: unknown = admin;

    for (const segment of key.split(".")) {
      if (
        UNSAFE_PATH_SEGMENTS.has(segment) ||
        !current ||
        typeof current !== "object" ||
        !(segment in current)
      ) {
        throw new Error(`Unknown admin text key: ${key}`);
      }
      current = (current as Record<string, unknown>)[segment];
    }

    return current;
  }

  function interpolateNamed(
    message: string,
    params: Record<string, InterpolationValue> | undefined,
    key: string,
  ): string {
    return message.replace(/\{([A-Za-z][A-Za-z0-9_]*)\}/g, (_match, name: string) => {
      if (!params || !(name in params)) {
        throw new Error(`Missing interpolation parameter "${name}" for ${key}`);
      }
      return String(params[name]);
    });
  }

  /** Resolve static Vietnamese administrator copy without runtime locale or network loading. */
  export function adminText<Key extends AdminTextKey>(
    key: Key,
    ...[params]: AdminTextArguments<Key>
  ): string {
    const leaf = resolveLeaf(key);

    if (typeof leaf === "string") {
      return interpolateNamed(
        leaf,
        params as Record<string, InterpolationValue> | undefined,
        key,
      );
    }

    if (typeof leaf === "function") {
      const formatterParams = params as { args?: readonly unknown[] } | undefined;
      const result = (leaf as (...args: readonly unknown[]) => unknown)(
        ...(formatterParams?.args ?? []),
      );
      return result == null ? "" : String(result);
    }

    throw new Error(`Admin text key does not resolve to a leaf: ${key}`);
  }
  ```

- [ ] **Step 2: Create `constants/admin-text.ts`.**

  ```typescript
  /** Stable administrator-copy entry point; locale catalogs stay replaceable. */
  export { admin as ADMIN_TEXT } from "@/i18n/catalog.admin.vi";
  ```

- [ ] **Step 3: Verify both files compile.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Only errors about `TEXT.ADMIN` in consumer files — no new errors.

---

### Task 5: Migrate `i18n/catalog.test.ts`

**Files:**
- Modify: `i18n/catalog.test.ts`

**Interfaces:**
- Consumes: `vi` from `@/i18n/catalog.vi`; `admin` from `@/i18n/catalog.admin.vi`; `text` from `@/i18n/text`; `adminText` from `@/i18n/admin-text`
- Produces: test suite that validates both catalogs

- [ ] **Step 1: Rewrite `i18n/catalog.test.ts` to cover both catalogs.**

  The existing tests check:
  1. Non-empty branches and leaves (structural validation).
  2. Typed path accessor resolution (`text("NAV.ARENA")`, `text("ADMIN.NAV.OVERVIEW")`, `text("COMMON.CLOSE")`).
  3. Named interpolation escaping.
  4. Formatter function callable through typed path.

  After the split, `text("ADMIN.NAV.OVERVIEW")` is no longer a valid `TextKey`. Replace with `adminText("NAV.OVERVIEW")`.

  ```typescript
  import assert from "node:assert/strict";
  import { test } from "vitest";

  import { vi } from "@/i18n/catalog.vi";
  import { admin } from "@/i18n/catalog.admin.vi";
  import { text } from "@/i18n/text";
  import { adminText } from "@/i18n/admin-text";

  function visitCatalog(
    value: unknown,
    path: string[] = [],
    seen = new Set<string>(),
  ): void {
    if (typeof value === "string") {
      assert.ok(value.trim().length > 0, `${path.join(".")} must not be empty`);
      assert.ok(!seen.has(path.join(".")), `${path.join(".")} is duplicated`);
      seen.add(path.join("."));
      return;
    }

    if (typeof value === "function") {
      assert.ok(!seen.has(path.join(".")), `${path.join(".")} is duplicated`);
      seen.add(path.join("."));
      return;
    }

    assert.ok(value && typeof value === "object", `${path.join(".")} is not a catalog branch`);
    for (const [key, child] of Object.entries(value)) {
      visitCatalog(child, [...path, key], seen);
    }
  }

  test("Vietnamese catalog contains only non-empty branches and leaves", () => {
    visitCatalog(vi);
  });

  test("Vietnamese admin catalog contains only non-empty branches and leaves", () => {
    visitCatalog(admin);
  });

  test("typed navigation labels resolve through public catalog paths", () => {
    assert.equal(text("NAV.ARENA"), vi.NAV.ARENA);
    assert.equal(text("COMMON.CLOSE"), vi.COMMON.CLOSE);
  });

  test("typed admin labels resolve through admin catalog paths", () => {
    assert.equal(adminText("NAV.OVERVIEW"), admin.NAV.OVERVIEW);
  });

  test("named interpolation returns markup-like values as plain text", () => {
    const result = text("COMMON.ITEM_COUNT", { count: "<script>" });

    assert.equal(result, "<script> mục");
  });

  test("formatter leaves remain callable through the typed path accessor", () => {
    assert.equal(text("COMMON.GO_TO_PAGE", { args: [3] }), "Đi đến trang 3");
  });
  ```

- [ ] **Step 2: Run tests to verify all pass.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run test
  ```
  Expected: All catalog tests pass; no `text("ADMIN...")` key errors.

---

### Task 6: Migrate the 12 ADMIN-ONLY consumer files

These files use `TEXT.ADMIN.` and nothing else from `TEXT`. Replace their `TEXT` import entirely with `ADMIN_TEXT`.

**Files (12 ADMIN-ONLY):**
- Modify: `app/admin/(dashboard)/problems/[id]/edit/page.tsx`
- Modify: `lib/admin/auxiliary-form.ts`
- Modify: `lib/auth/role-presentation.ts`
- Modify: `lib/operations/presentation.ts`
- Modify: `modules/admin/admin-access-state.tsx`
- Modify: `modules/admin/hooks/use-paginated-crud.ts`
- Modify: `modules/admin/notification-campaigns/campaign-cancel-dialog.tsx`
- Modify: `modules/admin/notification-campaigns/campaign-status.tsx`
- Modify: `modules/admin/operations/operation-progress.tsx`
- Modify: `modules/admin/permission-matrix.tsx`
- Modify: `modules/admin/submissions/admin-submission-detail-dialog.tsx`
- Modify: `modules/admin/user-role-review-dialog.tsx`

**Interfaces:**
- Consumes: `ADMIN_TEXT` from `@/constants/admin-text`
- Produces: same runtime behavior, no `TEXT.ADMIN.` references remaining

- [ ] **Step 1: For each file, apply the following transformation:**

  Find:
  ```typescript
  import { TEXT } from "@/constants/text";
  ```
  Replace with:
  ```typescript
  import { ADMIN_TEXT } from "@/constants/admin-text";
  ```

  Then globally replace `TEXT.ADMIN.` with `ADMIN_TEXT.` throughout the file.

  Apply to all 12 files listed above. Use exact string replacement — do not alter anything else.

- [ ] **Step 2: Verify the 12 files compile.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Errors only in the remaining BOTH files and `text("ADMIN...")` call sites.

---

### Task 7: Migrate the 57 BOTH consumer files

These files use both `TEXT.ADMIN.` and `TEXT.<other>`. They need to keep the `TEXT` import AND add `ADMIN_TEXT`.

**Files (57 BOTH, grouped for clarity):**

app/admin pages:
- `app/admin/(dashboard)/attribute-definitions/page.tsx`
- `app/admin/(dashboard)/contests/page.tsx`
- `app/admin/(dashboard)/difficulties/page.tsx`
- `app/admin/(dashboard)/elements/page.tsx`
- `app/admin/(dashboard)/levels/page.tsx`
- `app/admin/(dashboard)/materials/categories/page.tsx`
- `app/admin/(dashboard)/materials/page.tsx`
- `app/admin/(dashboard)/page.tsx`
- `app/admin/(dashboard)/problems/page.tsx`
- `app/admin/(dashboard)/progression/page.tsx`
- `app/admin/(dashboard)/ranks/page.tsx`
- `app/admin/(dashboard)/rarities/page.tsx`
- `app/admin/(dashboard)/roles/page.tsx`
- `app/admin/(dashboard)/tags/page.tsx`
- `app/admin/(dashboard)/traits/page.tsx`
- `app/admin/(dashboard)/users/page.tsx`

lib/ helpers:
- `lib/notifications/campaign-presentation.ts`

modules/admin:
- `modules/admin/admin-data-table.tsx`
- `modules/admin/attribute-lifecycle-review-dialog.tsx`
- `modules/admin/attribute-revision-history-sheet.tsx`
- `modules/admin/audit/admin-audit-explorer.tsx`
- `modules/admin/audit/admin-recent-audit.tsx`
- `modules/admin/audit/audit-detail-sheet.tsx`
- `modules/admin/contest-overview/admin-contest-overview.tsx`
- `modules/admin/dialogs/attribute-definition-dialog.tsx`
- `modules/admin/dialogs/contest-dialog.tsx`
- `modules/admin/dialogs/create-user-dialog.tsx`
- `modules/admin/dialogs/difficulty-dialog.tsx`
- `modules/admin/dialogs/edit-role-dialog.tsx`
- `modules/admin/dialogs/element-dialog.tsx`
- `modules/admin/dialogs/level-dialog.tsx`
- `modules/admin/dialogs/material-category-dialog.tsx`
- `modules/admin/dialogs/material-dialog.tsx`
- `modules/admin/dialogs/rank-dialog.tsx`
- `modules/admin/dialogs/rarity-dialog.tsx`
- `modules/admin/dialogs/role-dialog.tsx`
- `modules/admin/dialogs/tag-dialog.tsx`
- `modules/admin/dialogs/trait-dialog.tsx`
- `modules/admin/fields/admin-related-entity-fields.tsx`
- `modules/admin/judge/admin-judge-operations.tsx`
- `modules/admin/material-lifecycle-actions.tsx`
- `modules/admin/notification-campaigns/admin-notification-campaigns.tsx`
- `modules/admin/notification-campaigns/campaign-composer-dialog.tsx`
- `modules/admin/notification-campaigns/campaign-detail-sheet.tsx`
- `modules/admin/notification-campaigns/campaign-review-dialog.tsx`
- `modules/admin/observability/admin-observability.tsx`
- `modules/admin/operations/admin-operations.tsx`
- `modules/admin/operations/operation-batch-history.tsx`
- `modules/admin/operations/operation-control-dialog.tsx`
- `modules/admin/operations/operation-detail-sheet.tsx`
- `modules/admin/problem-form.tsx`
- `modules/admin/reward-profiles/admin-reward-profile.tsx`
- `modules/admin/role-metadata-review-dialog.tsx`
- `modules/admin/role-policy-review-dialog.tsx`
- `modules/admin/submissions/admin-submission-monitor.tsx`
- `modules/admin/user-invitation-resend-dialog.tsx`
- `modules/admin/user-lifecycle-dialog.tsx`

e2e:
- `e2e/admin.spec.ts`

**Interfaces:**
- Consumes: `TEXT` from `@/constants/text` (kept), `ADMIN_TEXT` from `@/constants/admin-text` (added)
- Produces: same runtime behavior, no `TEXT.ADMIN.` references remaining

- [ ] **Step 1: For each file, add the ADMIN_TEXT import and replace TEXT.ADMIN usages.**

  For each file that already has:
  ```typescript
  import { TEXT } from "@/constants/text";
  ```
  Add on the line immediately after (or in the same import block):
  ```typescript
  import { ADMIN_TEXT } from "@/constants/admin-text";
  ```
  Then globally replace `TEXT.ADMIN.` with `ADMIN_TEXT.` throughout the file.

  Note: Some files import TEXT alongside other named imports:
  ```typescript
  import { TEXT } from "@/constants/text";
  ```
  Just add a second import statement — do not merge into one line.

  For `e2e/admin.spec.ts`: same pattern. This is a Playwright test file; treat it the same.

- [ ] **Step 2: Run tsc to confirm no BOTH-file errors remain.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Only errors from the 8 `text("ADMIN...")` call sites remain.

---

### Task 8: Migrate the 8 `text("ADMIN...")` call-site files

These files call `text("ADMIN.X.Y", ...)` and need to switch to `adminText("X.Y", ...)`.

**Files:**
- Modify: `components/protected-content.tsx` — `text("ADMIN.ACCESS.FORBIDDEN_TITLE")` → `adminText("ACCESS.FORBIDDEN_TITLE")`
- Modify: `constants/admin-navigation.ts` — 20 calls like `text("ADMIN.NAV.OVERVIEW")` → `adminText("NAV.OVERVIEW")`
- Modify: `i18n/catalog.test.ts` — already migrated in Task 5
- Modify: `modules/admin/admin-brand.tsx` — check which ADMIN text calls exist
- Modify: `modules/admin/admin-header.tsx` — check which ADMIN text calls exist
- Modify: `modules/admin/admin-shell.tsx` — check which ADMIN text calls exist
- Modify: `modules/admin/admin-sidebar-nav.tsx` — check which ADMIN text calls exist
- Modify: `modules/admin/admin-sidebar.tsx` — check which ADMIN text calls exist

**Interfaces:**
- Consumes: `adminText` from `@/i18n/admin-text`
- Produces: same runtime behavior, no `text("ADMIN...")` calls remaining

- [ ] **Step 1: For each file, replace `text("ADMIN.` with `adminText("`  and strip the `ADMIN.` prefix from keys.**

  The key transformation is: `text("ADMIN.X.Y.Z", params)` → `adminText("X.Y.Z", params)`.

  For imports:
  - If the file ONLY uses `text` for ADMIN keys, replace the entire import:
    ```typescript
    import { text } from "@/i18n/text";
    ```
    with:
    ```typescript
    import { adminText } from "@/i18n/admin-text";
    ```
  - If the file uses `text` for BOTH admin and non-admin keys, add `adminText` to the existing import while keeping `text`:
    ```typescript
    import { text } from "@/i18n/text";
    import { adminText } from "@/i18n/admin-text";
    ```

  **`components/protected-content.tsx`** uses `text("ADMIN.ACCESS.FORBIDDEN_TITLE")` and `text("COMMON.LOADING")` and `text("COMMON.ERROR")` — it needs BOTH imports.

  **`constants/admin-navigation.ts`** — check if it uses any non-ADMIN text keys. If not, replace import entirely.

  For `modules/admin/admin-brand.tsx`, `admin-header.tsx`, `admin-shell.tsx`, `admin-sidebar-nav.tsx`, `admin-sidebar.tsx` — read each file before editing to confirm the exact call sites.

- [ ] **Step 2: Run full tsc to confirm zero errors.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npx tsc --noEmit
  ```
  Expected: Zero TypeScript errors.

---

### Task 9: Run full verification suite

**Files:** None created; verification only.

- [ ] **Step 1: Run all checks.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run verify
  ```
  Expected: All checks green — `check:architecture`, `eslint`, `check:routes`, `check:user-role-contract`, `test:contracts`, `vitest`, `tsc --noEmit`.

- [ ] **Step 2: Add `constants/admin-text.ts` to the architecture checker's `MESSAGE_CATALOG_FILES` set.**

  Open `scripts/check-frontend-architecture.mjs` line 42:
  ```javascript
  const MESSAGE_CATALOG_FILES = new Set(["constants/text.ts"]);
  ```
  Change to:
  ```javascript
  const MESSAGE_CATALOG_FILES = new Set(["constants/text.ts", "constants/admin-text.ts"]);
  ```

  This prevents the architecture checker from flagging Vietnamese strings in `constants/admin-text.ts`. (It is a re-export file with no string literals, so this may be a no-op, but it establishes the correct semantic role.)

  **Wait** — re-read the constraint: "Do NOT raise the budget, weaken, or edit `website/scripts/check-bundle.mjs`." That is `check-bundle.mjs`, not `check-frontend-architecture.mjs`. The architecture checker is a different script. However, the `constants/admin-text.ts` file only contains `export { admin as ADMIN_TEXT }` — no Vietnamese strings — so the architecture checker will not flag it. Skip this step; no edit needed.

- [ ] **Step 3: Run lint and confirm no architecture violations.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run lint
  ```
  Expected: Zero new violations.

---

### Task 10: Build and measure bundle reduction

**Files:** None modified; measurement only.

- [ ] **Step 1: Run the production build.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && npm run build
  ```
  Expected: Build completes with no errors.

- [ ] **Step 2: Measure the "/" route client chunk size.**

  Run:
  ```bash
  node -e "const fs=require('fs');const k='__RSC_MANIFEST[\"/page\"]=';const s=fs.readFileSync('.next/server/app/page_client-reference-manifest.js','utf8');const m=JSON.parse(s.slice(s.indexOf(k)+k.length).trim().replace(/;$/,''));const c=new Set();for(const v of Object.values(m.clientModules))for(const x of v.chunks||[])if(x.endsWith('.js'))c.add(x);const z=require('zlib');let t=0;for(const x of c)t+=z.gzipSync(fs.readFileSync('.next/'+x),{level:9}).length;console.log('/ client chunks gz',t,'count',c.size)"
  ```

  The before value was ~45963 gzip bytes just for the catalog chunk. After the split, this chunk should no longer appear in the "/" manifest. Report before/after numbers.

- [ ] **Step 3: Verify `check-bundle.mjs` passes.**

  Run:
  ```bash
  cd /Users/jerry/nhanhmc/coding/Judgify/website && node scripts/check-bundle.mjs
  ```
  Expected: The "/" route sum is now under the 200 KB budget.

---

## Self-Review

**Spec coverage check:**

1. Move ADMIN namespace into `i18n/catalog.admin.vi.ts` → Task 1 ✓
2. `catalog.vi.ts` does NOT import admin module → Task 2 (vi is stripped) ✓
3. `constants/admin-text.ts` re-export → Task 4 ✓
4. Typed admin accessor `adminText` in `i18n/admin-text.ts` → Task 4 ✓
5. `i18n/types.ts` generic — `LeafPaths`/`PathValue` generic over catalog type → Task 3 ✓
6. `TextKey`/`ParamsFor` still work via `typeof vi` → Task 3 ✓
7. 69 admin files: `TEXT.ADMIN.` → `ADMIN_TEXT.` → Tasks 6 + 7 ✓
8. 14 `text("ADMIN...")` → `adminText("...")` → Tasks 5 + 8 ✓
9. `catalog.test.ts` updated for both catalogs → Task 5 ✓
10. Bundle proof (admin catalog not in "/" chunks) → Task 10 ✓

**Potential gap — `e2e/admin.spec.ts`:** The spec says "69 admin files." The grep found it in the BOTH list. Confirm it truly uses `TEXT.ADMIN.` (not just in comments) before editing it; Playwright e2e test files behave the same as source for this migration.

**Placeholder scan:** No TBDs, no "implement later", all code blocks are complete.

**Type consistency:**
- `AdminTextKey` defined in Task 3, consumed in Task 4 — consistent.
- `AdminTextArguments` defined in Task 3, consumed in Task 4 — consistent.
- `adminText` defined in Task 4, consumed in Tasks 5, 8 — consistent.
- `ADMIN_TEXT` defined in Task 4, consumed in Tasks 6, 7 — consistent.

No issues found.
