# Judgify Frontend Product Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn every Judgify route into one fast, accessible, responsive Vietnamese product using real typed APIs, secure cookie sessions, one WebSocket per tab, centralized copy, and polished light/dark/system themes.

**Architecture:** Next.js App Router remains server-first for public/read-heavy snapshots, while focused client islands own filters, editors, mutations, session coordination, and realtime. One API client, session provider, refresh coordinator, and WebSocket provider serve the whole product; domain modules consume typed contracts rather than mock arrays or local token decoding.

**Tech Stack:** Next.js 16.1.6, React 19.2.3, TypeScript 5, Tailwind CSS 4, Base UI/shadcn, next-themes, native Fetch, native WebSocket, Lucide icons, existing Framer Motion 12, KaTeX/Markdown packages, and dev-only browser/unit test tooling.

## Global Constraints

- Keep Next.js, React, Tailwind, Base UI/shadcn, and existing Framer Motion.
- Do not add another state framework, data-fetching framework, WebSocket library, UI kit, or animation runtime.
- Product-authored launch copy is Vietnamese and comes from one typed catalog.
- Domain/user/admin-authored content stays backend data and is not placed in the text catalog.
- `EntityID` is a UUID string everywhere; no `Number(params.id)` or numeric domain ID remains.
- Browser credentials use `credentials: "include"`; no access/refresh token or cached profile in `localStorage`.
- One WebSocket connection per browser tab; no EventSource/SSE and no final submission polling.
- Theme choices are light, dark, and system with no forced dark mode or hydration flash.
- Motion respects reduced-motion and is reserved for orientation/feedback, not decoration.
- Every route distinguishes loading, empty, unauthorized, forbidden, not found, retryable error, offline/reconnecting, and success.
- Every mutation has pending/disabled state, idempotency where required, optimistic conflict handling, and recovery after unknown outcome.
- No runtime mock or placeholder data in a production route.
- No blanket ESLint disable; baseline errors are fixed structurally.
- No commits.

---

## Product Design Direction: Judgify Fusion

- Dense judge/workbench surfaces use a neutral ink-and-slate base, crisp hierarchy, compact data rhythm, and strong focus states.
- Cultivation identity appears through restrained jade/amber accents, progression glyphs, and meaningful state transitions; it does not turn every screen into fantasy decoration.
- Light mode is first-class, not inverted dark mode. Dark mode preserves contrast without pure-black glare.
- Cards are used for grouped actions/status, not as a wrapper around every paragraph.
- Tables become cards only where mobile information hierarchy demands it.
- Existing Framer Motion handles route/list/dialog transitions; CSS handles hover/focus/theme transitions.

## Required Execution Order

The numeric task ID is authoritative: **1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13**. Task sections are grouped in this document for foundation and product review; an implementer must jump to and finish the next numeric ID before starting a later one.

### Task 1: Add Frontend Test Harness and Eliminate Baseline Lint/Build Defects

**Files:**
- Modify: `website/package.json`
- Modify: `website/package-lock.json`
- Create: `website/vitest.config.ts`
- Create: `website/test/setup.ts`
- Create: `website/playwright.config.ts`
- Create: `website/test/smoke.spec.ts`
- Modify: `website/app/(main)/about/page.tsx`
- Modify: `website/app/(main)/materials/[id]/page.tsx`
- Modify: `website/app/admin/(dashboard)/layout.tsx`
- Modify: `website/app/admin/(dashboard)/roles/page.tsx`
- Modify: `website/components/theme-toggle.tsx`
- Modify: `website/hooks/use-contest-sse.ts`
- Modify: `website/modules/admin/dialogs/contest-dialog.tsx`
- Modify: `website/modules/admin/dialogs/difficulty-dialog.tsx`
- Modify: `website/modules/admin/dialogs/element-dialog.tsx`
- Modify: `website/modules/admin/dialogs/level-dialog.tsx`
- Modify: `website/modules/admin/dialogs/material-category-dialog.tsx`
- Modify: `website/modules/admin/dialogs/material-dialog.tsx`
- Modify: `website/modules/admin/dialogs/rank-dialog.tsx`
- Modify: `website/modules/admin/dialogs/rarity-dialog.tsx`
- Modify: `website/modules/admin/dialogs/role-dialog.tsx`
- Modify: `website/modules/admin/dialogs/tag-dialog.tsx`
- Modify: `website/modules/admin/dialogs/trait-dialog.tsx`
- Modify: `website/modules/admin/hooks/use-paginated-crud.ts`
- Modify: `website/modules/admin/problem-form.tsx`
- Modify: `website/modules/arena/hooks/use-arena-filters.ts`
- Modify: `website/modules/contest/contest-standings-table.tsx`
- Modify: `website/modules/layout/header.tsx`
- Modify: `website/modules/ranking/ranking-spiritual-root-grid.tsx`

**Interfaces:**
- Produces `npm run test`, `npm run test:coverage`, and `npm run test:e2e` without adding runtime production dependencies.

- [ ] **Step 1: Add failing smoke and lint-baseline tests**

The first browser smoke opens `/`, `/arena`, `/contest`, `/materials`, `/ranking`, `/login`, and `/admin/login`, asserts no uncaught error, one main landmark, and a visible page heading. Unit setup uses jsdom only for client components; pure modules use Node environment.

- [ ] **Step 2: Run current gates and retain exact failures**

Run:

```bash
cd website
npm run lint
npm run build
```

Expected baseline: lint reports 27 errors/7 warnings. Build must be run with a bounded CI timeout and diagnostic env; if it again remains at optimized build, capture process/build trace and diagnose rather than calling it pass.

- [ ] **Step 3: Fix structural lint failures**

Apply these exact patterns:

- replace authored quote literals with catalog text or typographic entities;
- remove effect-driven initial/reset state by deriving state, moving reset into event handlers, or remounting dialogs with `key`;
- replace `any` in `use-paginated-crud.ts` with generic request/error constraints;
- remove the SSE hook rather than suppressing its recursive callback lint error once WebSocket provider lands; until then isolate it behind a temporary non-production file;
- replace unused declarations and `<img>` with `next/image` where size/loader is known;
- theme toggle uses a mounted-safe client boundary rendered by the provider, not synchronous `setMounted` in effect.

- [ ] **Step 4: Diagnose and fix build non-termination**

Run:

```bash
NEXT_TELEMETRY_DISABLED=1 npm run build
```

Inspect font/network imports, async server fetches without deadline, recursive route generation, and Turbopack trace. Local product build must not depend on an unavailable external network resource. Use self-hosted/system font declarations if a remote font blocks build.

- [ ] **Step 5: Verify the checkpoint**

Run:

```bash
cd website
npm run lint
npm run test
npm run build
git diff --check
```

Expected: zero lint errors/warnings, tests pass, build exits 0. Do not commit.

### Task 2: Implement Three-Layer Design Tokens and Light/Dark/System Themes

**Files:**
- Modify: `website/app/globals.css`
- Modify: `website/app/layout.tsx`
- Modify: `website/components/theme-provider.tsx`
- Modify: `website/components/theme-toggle.tsx`
- Modify: `website/components/app-providers.tsx`
- Modify: `website/components/ui/avatar.tsx`
- Modify: `website/components/ui/badge.tsx`
- Modify: `website/components/ui/button.tsx`
- Modify: `website/components/ui/card.tsx`
- Modify: `website/components/ui/dialog.tsx`
- Modify: `website/components/ui/dropdown-menu.tsx`
- Modify: `website/components/ui/input.tsx`
- Modify: `website/components/ui/label.tsx`
- Modify: `website/components/ui/select.tsx`
- Modify: `website/components/ui/separator.tsx`
- Modify: `website/components/ui/sheet.tsx`
- Modify: `website/components/ui/switch.tsx`
- Modify: `website/components/ui/table.tsx`
- Modify: `website/components/ui/tabs.tsx`
- Modify: `website/components/ui/textarea.tsx`
- Modify: `website/components/ui/tooltip.tsx`
- Modify: `website/constants/styles.ts`
- Modify: `website/lib/utils.ts`
- Create: `website/design/tokens.ts`
- Create: `website/components/ui/theme-preview.tsx`
- Create: `website/test/theme.test.tsx`

**Interfaces:**

```ts
export type ThemePreference = "light" | "dark" | "system";
```

- [ ] **Step 1: Write theme/token tests**

Assert three choices, default system, persisted non-sensitive preference, no forced theme, SSR script/class avoids flash, semantic tokens exist in both modes, focus/ring/error/success/contest/cultivation colors meet contrast, and reduced-motion media disables nonessential transitions.

- [ ] **Step 2: Run focused tests**

Run: `cd website && npm run test -- theme.test.tsx`

Expected: FAIL because current provider forces dark/mounted workaround.

- [ ] **Step 3: Implement primitive → semantic → component variables**

Define primitive neutral/jade/amber/red/blue scales, semantic variables such as:

```css
--background
--foreground
--surface
--surface-raised
--border
--muted
--accent
--accent-foreground
--focus-ring
--success
--warning
--danger
--code-background
--editor-gutter
```

Component classes consume semantic tokens only. Use `color-scheme` and next-themes `attribute="class"`, `defaultTheme="system"`, `enableSystem`, and `disableTransitionOnChange`.

- [ ] **Step 4: Normalize base components**

Update button/input/card/dialog/table/tabs/badge/tooltip/sheet styles for height, radius, focus-visible, disabled, density, and touch target consistency in both themes.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test -- theme.test.tsx
npm run lint
npm run build
```

Expected: all exit 0 with no hydration warning. Do not commit.

### Task 3: Replace Text Constants with a Typed Vietnamese Catalog

**Files:**
- Replace: `website/constants/text.ts`
- Create: `website/i18n/catalog.vi.ts`
- Create: `website/i18n/types.ts`
- Create: `website/i18n/text.ts`
- Create: `website/i18n/catalog.test.ts`
- Modify: `website/app/layout.tsx`
- Modify: `website/app/page.tsx`
- Modify: `website/app/(auth)/layout.tsx`
- Modify: `website/app/(main)/layout.tsx`
- Modify: `website/app/(main)/about/page.tsx`
- Modify: `website/components/app-toaster.tsx`
- Modify: `website/components/confirm-dialog.tsx`
- Modify: `website/components/loading-spinner.tsx`
- Modify: `website/components/pagination-controls.tsx`
- Modify: `website/components/protected-content.tsx`
- Modify: `website/lib/toast.ts`
- Modify: `website/modules/layout/app-shell.tsx`
- Modify: `website/modules/layout/header.tsx`
- Modify: `website/constants/navigation.ts`
- Modify: `website/constants/admin-navigation.ts`

Route- and domain-specific literal migrations continue through the exact files enumerated in Tasks 8–12; this task owns the catalog and common surfaces.

**Interfaces:**

```ts
export const vi = {
  common: {
    actions: { save: "Lưu", cancel: "Hủy", retry: "Thử lại" },
    states: { loading: "Đang tải", empty: "Chưa có dữ liệu" },
  },
  auth: {},
  arena: {},
  contest: {},
  submission: {},
  cultivation: {},
  materials: {},
  profile: {},
  ranking: {},
  admin: {},
  errors: {},
} as const;

export type TextKey = LeafPaths<typeof vi>;
export function text<K extends TextKey>(key: K, params?: ParamsFor<K>): string;
```

- [ ] **Step 1: Write catalog completeness/interpolation tests**

Assert no duplicate/missing key, all leaves are non-empty strings/functions, placeholder names match typed parameters, no HTML is returned, and route/navigation/admin labels resolve through keys.

- [ ] **Step 2: Run focused tests**

Run: `cd website && npm run test -- catalog.test.ts`

Expected: FAIL because typed catalog does not exist.

- [ ] **Step 3: Implement typed lookup and interpolation**

Use a static imported Vietnamese object, compile-time leaf paths, and named interpolation with escaping. Do not add runtime locale detection/network loading. Domain content, usernames, problem/material text, contest announcements, and admin-authored labels remain DTO fields.

- [ ] **Step 4: Migrate product literals route-by-route**

Replace headings, buttons, form labels/help, status copy, toasts, errors, empty/loading/reconnect text, aria labels, navigation, and confirmations. Add a static scan test that allows literals only in catalog, tests, backend-domain data fixtures, and technical attributes.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test -- catalog.test.ts
npm run lint
npm run build
```

Expected: all exit 0. Do not commit.

### Task 7: Add One Native WebSocket Provider with Replay and REST Resync

**Files:**
- Create: `website/lib/realtime/envelope.ts`
- Create: `website/lib/realtime/socket.ts`
- Create: `website/lib/realtime/subscription.ts`
- Create: `website/lib/realtime/socket.test.ts`
- Create: `website/contexts/realtime-context.tsx`
- Create: `website/contexts/realtime-context.test.tsx`
- Modify: `website/components/app-providers.tsx`
- Remove: `website/hooks/use-contest-sse.ts`

**Interfaces:**

```ts
export type RealtimeEnvelope<T = unknown> = {
  v: 1;
  op: string;
  id?: string;
  cursor?: string;
  topic?: string;
  type: string;
  aggregate_version?: number;
  occurred_at?: string;
  data?: T;
};

type RealtimeState = "idle" | "connecting" | "connected" | "reconnecting" | "offline";

type Subscription<T> = {
  topic: string;
  snapshot(): Promise<{ data: T; cursor: string; aggregateVersion: number }>;
  apply(current: T, envelope: RealtimeEnvelope): T;
};
```

- [ ] **Step 1: Write connection/order/replay tests**

Prove one socket per provider/tab, reference-counted topics, auth/anonymous same endpoint, reconnect jitter cap, cursor persistence only in memory/bounded session record, duplicate/stale ignored, exactly local+1 applied, gap triggers snapshot resync, `resync_required` reload, 1013 reconnect+resync, offline state, malformed/unknown event rejection, and unsubscribe cleanup.

- [ ] **Step 2: Run focused tests**

Run: `cd website && npm run test -- socket.test.ts realtime-context.test.tsx`

Expected: FAIL because provider does not exist.

- [ ] **Step 3: Implement native socket lifecycle**

Create WebSocket only in the provider, serialize outbound subscribe/unsubscribe, validate every envelope at boundary, track last cursor and per-aggregate versions, and derive reconnect delay with bounded jitter. Never retry a rejected unauthorized topic indefinitely.

- [ ] **Step 4: Implement snapshot fences in hooks**

On first subscriber: fetch REST snapshot+cursor, subscribe/replay after cursor, buffer events during snapshot, then apply contiguous versions. On gap: pause local application, refetch canonical snapshot, replace state, and resume from new cursor.

- [ ] **Step 5: Verify and remove SSE**

Run:

```bash
cd website
npm run test -- socket.test.ts realtime-context.test.tsx
rg -n 'EventSource|use-contest-sse|text/event-stream' .
npm run lint
npm run build
```

Expected: tests/build pass and no production SSE match remains. Do not commit.

### Task 8: Rebuild the Responsive Application and Admin Shells

**Files:**
- Modify: `website/modules/layout/app-shell.tsx`
- Modify: `website/modules/layout/header.tsx`
- Modify: `website/app/(main)/layout.tsx`
- Modify: `website/modules/admin/admin-shell.tsx`
- Modify: `website/modules/admin/admin-header.tsx`
- Modify: `website/modules/admin/admin-sidebar.tsx`
- Modify: `website/modules/admin/admin-sidebar-nav.tsx`
- Modify: `website/app/admin/(dashboard)/layout.tsx`
- Modify: `website/constants/navigation.ts`
- Modify: `website/constants/admin-navigation.ts`
- Create: `website/components/page-state.tsx`
- Create: `website/components/skip-link.tsx`
- Create: `website/test/shells.spec.ts`

**Interfaces:**
- Produces capability-filtered responsive navigation, persistent desktop work areas, mobile sheets, keyboard skip/focus behavior, and shared page-state presentation.

- [ ] **Step 1: Write shell accessibility/responsive tests**

Assert one header/nav/main, skip link, logical tab order, visible focus, active route, capability-filtered admin items, no auth effect redirect loop, 44px mobile targets, mobile sheet focus trap/restore, desktop density, reduced motion, and all page-state variants.

- [ ] **Step 2: Run browser test**

Run: `cd website && npm run test:e2e -- shells.spec.ts`

Expected: FAIL against current shells.

- [ ] **Step 3: Implement app shell**

Use max-width content for reading/catalog pages, full-width bounded workbench for problem/admin tables, sticky header only where it preserves content, and a clear primary action region. Navigation labels use catalog keys; session/capabilities come from providers.

- [ ] **Step 4: Implement admin shell**

Desktop uses collapsible sidebar and command-oriented header; mobile uses accessible sheet. Group navigation by Overview, Users & Access, Content, Judge, Contest, Cultivation, Notifications, Operations, Audit. Hide inaccessible links but still rely on backend authorization.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test:e2e -- shells.spec.ts
npm run lint
npm run build
```

Expected: all exit 0. Do not commit.

### Task 9: Convert Public Catalog, Arena, and Problem Workspace to Real Data

**Files:**
- Modify: `website/app/page.tsx`
- Modify: `website/app/(main)/about/page.tsx`
- Modify: `website/app/(main)/arena/page.tsx`
- Modify: `website/app/(main)/arena/arena-client.tsx`
- Modify: `website/app/(main)/arena/[id]/page.tsx`
- Modify: `website/modules/arena/components/arena-filter-bar.tsx`
- Modify: `website/modules/arena/components/difficulty-badge.tsx`
- Modify: `website/modules/arena/components/problem-table.tsx`
- Modify: `website/modules/arena/hooks/use-arena-filters.ts`
- Modify: `website/modules/arena/problem-description-panel.tsx`
- Modify: `website/modules/problem/file-submission.tsx`
- Modify: `website/modules/problem/submission-detail.tsx`
- Modify: `website/modules/problem/submission-history.tsx`
- Modify: `website/modules/problem/test-case-block.tsx`
- Modify: `website/modules/shared/markdown-renderer.tsx`
- Modify: `website/services/problem.service.ts`
- Modify: `website/services/submission.service.ts`
- Modify: `website/types/problem.ts`
- Modify: `website/types/submission.ts`
- Create: `website/test/arena.spec.ts`
- Create: `website/test/problem.spec.ts`

**Interfaces:**
- Consumes public problem snapshots and authenticated submission APIs.
- Produces server-rendered list/detail plus client filter/editor/submission/realtime islands.

- [ ] **Step 1: Write route capability tests**

Assert server initial catalog, URL-backed search/difficulty/tag/sort/page, UUID detail, safe rendered statement, no hidden data, loading/empty/error/not-found, editor language/source limits, idempotent submit unknown outcome, separate processing phase/active verdict, per-test visibility, source ownership, and realtime gap resync.

- [ ] **Step 2: Run browser tests**

Run: `cd website && npm run test:e2e -- arena.spec.ts problem.spec.ts`

Expected: FAIL while mock/numeric/polling behavior remains.

- [ ] **Step 3: Implement server-first catalog/detail**

Use server fetch with explicit cache/no-store policy from the spec. Parse search params through typed allowlists and return canonical pagination. Problem statement renders backend-sanitized HTML/artifact under a strict component boundary; do not re-enable raw authored HTML.

- [ ] **Step 4: Implement focused problem client**

Editor/submission/history/detail use real services. Show compile/run phase and active verdict independently. Persist draft source only under explicit bounded non-sensitive local draft policy; never cache hidden tests or auth state.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test:e2e -- arena.spec.ts problem.spec.ts
rg -n 'mock|placeholder|setInterval|Number\\(params\\.id\\)' app/'(main)'/arena modules/arena modules/problem services/problem.service.ts services/submission.service.ts
npm run lint
npm run build
```

Expected: no runtime mock/polling/numeric ID; all gates pass. Do not commit.

### Task 10: Convert Login, Registration, Recovery, Profile, and Session Security

**Files:**
- Modify: `website/app/(auth)/login/page.tsx`
- Modify: `website/app/(auth)/register/page.tsx`
- Modify: `website/app/(auth)/forgot-password/page.tsx`
- Modify: `website/app/(auth)/reset-password/page.tsx`
- Modify: `website/app/admin/login/page.tsx`
- Modify: `website/modules/auth/AuthContainer.tsx`
- Modify: `website/modules/auth/LoginForm.tsx`
- Modify: `website/modules/auth/RegisterFlow.tsx`
- Modify: `website/modules/auth/sections/personal-info-section.tsx`
- Modify: `website/modules/auth/sections/trait-selection-section.tsx`
- Modify: `website/app/(main)/profile/page.tsx`
- Modify: `website/modules/profile/cultivation-panel.tsx`
- Modify: `website/modules/profile/stats-panel.tsx`
- Modify: `website/modules/profile/tags-panel.tsx`
- Modify: `website/modules/cultivation/trait-card.tsx`
- Modify: `website/modules/cultivation/trait-codex-modal.tsx`
- Modify: `website/services/auth.service.ts`
- Modify: `website/services/user.service.ts`
- Create: `website/modules/profile/session-security.tsx`
- Create: `website/modules/profile/recovery-contact.tsx`
- Create: `website/test/auth.spec.ts`
- Create: `website/test/profile.spec.ts`

**Interfaces:**
- Consumes pre-session/trait offers/recoverable issuance/OAuth/recovery/session/profile APIs.

- [ ] **Step 1: Write complete auth/onboarding tests**

Cover anonymous CSRF bootstrap, login idempotency UUID reused after unknown outcome, generic forgot response, reset single use, registration recovery email, initial offer/reroll quota, exact root+three-of-six selection, lower-sequence late offer ignored, reload recovery, OAuth start/callback/finalize, no callback cookie session, session list/revoke/logout-all, recent reauth, recovery verify/resend/replace/remove, typed profile visibility/version conflicts, and no sensitive storage.

- [ ] **Step 2: Run browser tests**

Run: `cd website && npm run test:e2e -- auth.spec.ts profile.spec.ts`

Expected: FAIL until new APIs replace current flow.

- [ ] **Step 3: Implement unified auth flow**

Login and `/admin/login` share one form/session system; safe `next` accepts same-origin paths only. Registration is a staged accessible flow: account+recovery → profile → durable trait offer/selection → review/submit. Unknown outcomes reuse the same logical UUID and show recover/retry state.

- [ ] **Step 4: Implement account security**

Profile presents canonical session/device list, revoke actions, logout all, recovery contact state/actions, OAuth identities/link/unlink, recent-reauth dialog, and typed profile values. Sensitive actions show confirmation and server CID on error.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test:e2e -- auth.spec.ts profile.spec.ts
rg -n 'localStorage|access_token|refresh_token|GET.*/oauth/.*/start|traits/roll' app modules/auth modules/profile contexts services
npm run lint
npm run build
```

Expected: no sensitive/legacy flow match and all gates pass. Do not commit.

### Task 11: Convert Contest, Ranking, Materials, and Notifications

**Files:**
- Modify: `website/app/(main)/contest/page.tsx`
- Modify: `website/app/(main)/contest/[id]/page.tsx`
- Modify: `website/modules/contest/contest-card.tsx`
- Modify: `website/modules/contest/contest-hero-section.tsx`
- Modify: `website/modules/contest/contest-rating-table.tsx`
- Modify: `website/modules/contest/contest-standings-table.tsx`
- Modify: `website/services/contest.service.ts`
- Modify: `website/app/(main)/ranking/page.tsx`
- Modify: `website/modules/ranking/ranking-hero-section.tsx`
- Modify: `website/modules/ranking/ranking-spiritual-root-grid.tsx`
- Modify: `website/modules/ranking/ranking-top-list.tsx`
- Modify: `website/services/ranking.service.ts`
- Modify: `website/app/(main)/materials/page.tsx`
- Modify: `website/app/(main)/materials/[id]/page.tsx`
- Modify: `website/modules/materials/materials-article-card.tsx`
- Modify: `website/modules/materials/materials-category-sidebar.tsx`
- Modify: `website/modules/materials/materials-difficulty-filter.tsx`
- Modify: `website/modules/materials/materials-hero-section.tsx`
- Modify: `website/modules/materials/materials-search-bar.tsx`
- Modify: `website/services/material.service.ts`
- Create: `website/modules/notifications/notification-center.tsx`
- Create: `website/services/notification.service.ts`
- Create: `website/types/notification.ts`
- Create: `website/test/contest.spec.ts`
- Create: `website/test/materials-ranking-notifications.spec.ts`

**Interfaces:**
- Consumes REST snapshot+cursor and one shared realtime provider.

- [ ] **Step 1: Write page/realtime tests**

Contest tests cover catalog/detail, lifecycle, registration/capacity, standings/freeze, own live state, announcements/clarifications, rating history, reconnect/gap. Other tests cover material search/category/slug/safe body/related/view, ranking keyset filters, persisted notification list/count/read/read-all/archive, offline/reconnect, and no fake fallback.

- [ ] **Step 2: Run browser tests**

Run:

```bash
cd website
npm run test:e2e -- contest.spec.ts materials-ranking-notifications.spec.ts
```

Expected: FAIL while current partial/mock/SSE pages remain.

- [ ] **Step 3: Implement server snapshots and client islands**

Catalog/article/ranking initial views render on server with typed query params. Contest detail and notification center subscribe through shared provider after REST fence. Frozen standings never derive hidden/live values in the client.

- [ ] **Step 4: Apply product polish**

Use compact standings/table density, sticky rank/name columns where appropriate, clear freeze/reconnect banners, material reading typography, cultivation accent only for ranking identity, skeletons matching final geometry, and responsive card transformations that preserve information order.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test:e2e -- contest.spec.ts materials-ranking-notifications.spec.ts
rg -n 'EventSource|mock|placeholder|setInterval' app/'(main)'/contest app/'(main)'/materials app/'(main)'/ranking modules/contest modules/materials modules/ranking modules/notifications
npm run lint
npm run build
```

Expected: no runtime mock/SSE/polling match and all gates pass. Do not commit.

### Task 12: Rebuild the Complete Admin Product on Real APIs

**Files:**
- Modify: `website/app/admin/(dashboard)/layout.tsx`
- Modify: `website/app/admin/(dashboard)/page.tsx`
- Modify: `website/app/admin/(dashboard)/contests/page.tsx`
- Modify: `website/app/admin/(dashboard)/difficulties/page.tsx`
- Modify: `website/app/admin/(dashboard)/elements/page.tsx`
- Modify: `website/app/admin/(dashboard)/levels/page.tsx`
- Modify: `website/app/admin/(dashboard)/materials/page.tsx`
- Modify: `website/app/admin/(dashboard)/materials/categories/page.tsx`
- Modify: `website/app/admin/(dashboard)/problems/page.tsx`
- Modify: `website/app/admin/(dashboard)/problems/create/page.tsx`
- Modify: `website/app/admin/(dashboard)/problems/[id]/edit/page.tsx`
- Modify: `website/app/admin/(dashboard)/ranks/page.tsx`
- Modify: `website/app/admin/(dashboard)/rarities/page.tsx`
- Modify: `website/app/admin/(dashboard)/roles/page.tsx`
- Modify: `website/app/admin/(dashboard)/tags/page.tsx`
- Modify: `website/app/admin/(dashboard)/traits/page.tsx`
- Modify: `website/app/admin/(dashboard)/users/page.tsx`
- Modify: `website/modules/admin/admin-data-table.tsx`
- Modify: `website/modules/admin/admin-header.tsx`
- Modify: `website/modules/admin/admin-shell.tsx`
- Modify: `website/modules/admin/admin-sidebar-nav.tsx`
- Modify: `website/modules/admin/admin-sidebar.tsx`
- Modify: `website/modules/admin/data-table-shell.tsx`
- Modify: `website/modules/admin/dialogs/contest-dialog.tsx`
- Modify: `website/modules/admin/dialogs/create-user-dialog.tsx`
- Modify: `website/modules/admin/dialogs/difficulty-dialog.tsx`
- Modify: `website/modules/admin/dialogs/edit-role-dialog.tsx`
- Modify: `website/modules/admin/dialogs/element-dialog.tsx`
- Modify: `website/modules/admin/dialogs/level-dialog.tsx`
- Modify: `website/modules/admin/dialogs/material-category-dialog.tsx`
- Modify: `website/modules/admin/dialogs/material-dialog.tsx`
- Modify: `website/modules/admin/dialogs/rank-dialog.tsx`
- Modify: `website/modules/admin/dialogs/rarity-dialog.tsx`
- Modify: `website/modules/admin/dialogs/resource-management-dialog.tsx`
- Modify: `website/modules/admin/dialogs/role-dialog.tsx`
- Modify: `website/modules/admin/dialogs/tag-dialog.tsx`
- Modify: `website/modules/admin/dialogs/trait-dialog.tsx`
- Modify: `website/modules/admin/element-form-dialog.tsx`
- Modify: `website/modules/admin/hooks/use-paginated-crud.ts`
- Modify: `website/modules/admin/permission-matrix.tsx`
- Modify: `website/modules/admin/problem-form.tsx`
- Modify: `website/types/admin.ts`
- Modify: `website/services/contest.service.ts`
- Modify: `website/services/cultivation.service.ts`
- Modify: `website/services/difficulty.service.ts`
- Modify: `website/services/element.service.ts`
- Modify: `website/services/level.service.ts`
- Modify: `website/services/material.service.ts`
- Modify: `website/services/problem.service.ts`
- Modify: `website/services/rank.service.ts`
- Modify: `website/services/rarity.service.ts`
- Modify: `website/services/role.service.ts`
- Modify: `website/services/submission.service.ts`
- Modify: `website/services/tag.service.ts`
- Modify: `website/services/user.service.ts`
- Create: `website/services/audit.service.ts`
- Create: `website/services/operation.service.ts`
- Create: `website/services/authorization.service.ts`
- Create: `website/modules/admin/operation-progress.tsx`
- Create: `website/modules/admin/audit-table.tsx`
- Create: `website/modules/admin/policy-diff.tsx`
- Create: `website/test/admin.spec.ts`

**Interfaces:**
- Produces overview; users/sessions/access; problems/judge jobs; contests; cultivation; materials; notifications; operations; audit; and system health surfaces.

- [ ] **Step 1: Write capability and workflow browser tests**

Test navigation by capability, direct forbidden route, overview partial dependency error, bounded tables/filters/sorts, create/edit dialogs without effect-state lint, optimistic conflict reload, preview/diff/reason/confirmation, protected role invariants, user suspend/session revoke, problem publish/rejudge/cancel, progression adjustment, material publish, campaign schedule, operation pause/resume/cancel, audit CID search, and successful lost-response recovery.

- [ ] **Step 2: Run browser tests**

Run: `cd website && npm run test:e2e -- admin.spec.ts`

Expected: FAIL against current CRUD/resource/permission mock contracts.

- [ ] **Step 3: Replace role/resource UI with revisioned policy matrix**

Load resource/action registry, roles, normalized policy snapshot, and current revision. Matrix uses accessible table/grid semantics, sticky labels, keyboard toggles, explicit wildcard display, filter without hiding selected changes, server preview diff, required reason, expected revision, and conflict recovery. Do not expose raw Casbin internals as the primary UX.

- [ ] **Step 4: Standardize admin commands and forms**

Replace effect-populated dialog state by keyed form components initialized from props. A shared command shell manages preview/apply/idempotency/unknown outcome/CID/operation link but domain forms retain typed fields and validation. Destructive/bulk actions never execute from one unconfirmed click.

- [ ] **Step 5: Implement operations/audit observability**

Show durable status/counts/attempts/age, bounded error classes, pause/cancel eligibility, and exact operation IDs. Audit table filters by actor/action/resource/target/CID/time; redacted diffs never reconstruct hidden secrets.

- [ ] **Step 6: Verify**

Run:

```bash
cd website
npm run test:e2e -- admin.spec.ts
rg -n 'resource-management-dialog|mock|placeholder|: any|as any' app/admin modules/admin services
npm run lint
npm run build
```

Expected: no legacy permission resource-management/runtime mock/explicit-any match and all gates pass. Do not commit.

### Task 13: Enforce Accessibility, Performance, Motion, and No-Mock Release Gates

**Files:**
- Modify: `website/next.config.ts`
- Modify: `website/eslint.config.mjs`
- Create: `website/test/accessibility.spec.ts`
- Create: `website/test/performance.spec.ts`
- Create: `website/test/no-runtime-mocks.test.ts`
- Create: `website/test/text-catalog.test.ts`
- Create: `website/test/theme-routes.spec.ts`
- Create: `website/scripts/check-bundle.mjs`
- Create: `website/scripts/check-routes.mjs`
- Modify: `website/package.json`

**Interfaces:**
- Produces automated route/theme/state/accessibility/performance evidence.

- [ ] **Step 1: Write route inventory and forbidden-pattern gates**

Discover App Router pages and assert each is in the browser matrix. Scan production code for mock fixtures, `EventSource`, polling timers, numeric entity conversions, sensitive localStorage, literal product copy outside catalog, blanket ESLint disables, and unbounded animation loops.

- [ ] **Step 2: Write accessibility/theme tests**

At mobile/tablet/desktop and light/dark/system, test keyboard-only navigation, focus order/visibility, dialog/sheet focus trap/restore, labels/errors/aria-live, table semantics, skip links, contrast, zoom 200%, reduced motion, and no hydration mismatch.

- [ ] **Step 3: Write performance budgets**

Use production build and browser traces to enforce:

- route-specific JS budget from the approved frontend spec;
- no duplicate API/session/WebSocket client bundles;
- one WebSocket per tab;
- bounded list rendering and image sizes;
- no layout shift from skeleton/theme/font;
- public page LCP/INP/CLS budgets under the documented test profile.

Bundle script parses Next build output and fails on route budget regression instead of printing a warning.

- [ ] **Step 4: Run complete frontend verification**

Run:

```bash
cd website
npm run lint
npx tsc --noEmit
npm run test:coverage
npm run build
npm run test:e2e
npm run check:bundle
npm run check:routes
```

Expected: every command exits 0, all routes/themes/viewports are covered, and production scan reports no forbidden pattern. Do not commit.

- [ ] **Step 5: Perform manual UI review with the UI/UX skill criteria**

Inspect home, login/register, arena/problem, contest, material, ranking, profile/security, notifications, and every admin group at 375, 768, 1280, and 1536px in both themes. Verify hierarchy, density, touch targets, focus, empty/error/reconnect states, reduced motion, and Vietnamese wrapping. Capture screenshots as evidence; fix defects before acceptance.

## Acceptance Criteria Reference

- [ ] Baseline lint and build both exit 0 without suppressing valid rules.
- [ ] Every production-authored string is served by the typed Vietnamese catalog.
- [ ] Every domain ID remains a UUID string.
- [ ] Browser auth is cookie/CSRF/session based with no token decoding/storage.
- [ ] Unknown refresh/login/register outcomes reuse one logical idempotency UUID.
- [ ] Exactly one native WebSocket exists per tab and no SSE/polling path remains.
- [ ] Every route uses a real backend API and explicit page states.
- [ ] Light, dark, and system work without flash across all routes.
- [ ] Admin policy UI uses registry + revisioned complete preview/apply.
- [ ] All admin domains, operations, and audit are usable on desktop and mobile.
- [ ] Keyboard, reduced motion, contrast, zoom, and screen-reader behaviors pass.
- [ ] Bundle and Web Vitals budgets pass production build checks.
- [ ] No additional animation runtime or UI/state framework was added.
- [ ] No commit was created.


### Task 4: Define Canonical UUID/API/Error Contracts

**Files:**
- Replace: `website/types/api.ts`
- Modify: `website/types/admin.ts`
- Modify: `website/types/auth.ts`
- Modify: `website/types/contest.ts`
- Modify: `website/types/cultivation.ts`
- Modify: `website/types/difficulty.ts`
- Modify: `website/types/material.ts`
- Modify: `website/types/problem.ts`
- Modify: `website/types/ranking.ts`
- Modify: `website/types/submission.ts`
- Modify: `website/types/tag.ts`
- Modify: `website/types/user.ts`
- Modify: `website/lib/format.ts`
- Create: `website/lib/api/contracts.ts`
- Create: `website/lib/api/error.ts`
- Create: `website/lib/api/schema.ts`
- Create: `website/lib/api/contracts.test.ts`
- Modify: `website/constants/api/auth.ts`
- Modify: `website/constants/api/base.ts`
- Modify: `website/constants/api/contest.ts`
- Modify: `website/constants/api/cultivation.ts`
- Modify: `website/constants/api/identity.ts`
- Modify: `website/constants/api/index.ts`
- Modify: `website/constants/api/material.ts`
- Modify: `website/constants/api/problem.ts`

**Interfaces:**

```ts
declare const entityIDBrand: unique symbol;
export type EntityID = string & { readonly [entityIDBrand]: "EntityID" };

export type ApiEnvelope<T> = {
  data: T;
  meta: { cid: string; pagination?: PaginationMeta };
};

export type ApiErrorEnvelope = {
  error: {
    code: string;
    params?: Record<string, string | number | boolean>;
    fields?: Record<string, string>;
  };
  meta: { cid: string };
};
```

- [ ] **Step 1: Write runtime parsing tests**

Reject numeric/malformed IDs, missing CID, duplicate/unknown sensitive fields, invalid timestamps/enums/pagination/cursor, unsafe error payload, and backend contract drift. Preserve UUID as string end-to-end.

- [ ] **Step 2: Run focused tests**

Run: `cd website && npm run test -- contracts.test.ts`

Expected: FAIL against current loose/numeric types.

- [ ] **Step 3: Implement one checked contract boundary**

Generate or check in API schema from the backend; validate network data once at the client/server boundary and expose typed DTOs. `ApiError` stores machine code, safe params/fields, HTTP status, retryability, and CID; UI maps copy through the catalog.

- [ ] **Step 4: Migrate service/entity types**

Replace all `number` entity IDs and `Number(params.id)` with validated `EntityID`. Remove duplicate response wrappers and backend entity assumptions from service files.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test -- contracts.test.ts
npx tsc --noEmit
npm run lint
```

Expected: no numeric domain ID remains. Do not commit.

### Task 5: Replace the API Client with Cookie, CSRF, Timeout, and Idempotency Support

**Files:**
- Create: `website/lib/api/client.ts`
- Create: `website/lib/api/csrf.ts`
- Create: `website/lib/api/idempotency.ts`
- Create: `website/lib/api/client.test.ts`
- Modify: `website/services/auth.service.ts`
- Modify: `website/services/contest.service.ts`
- Modify: `website/services/cultivation.service.ts`
- Modify: `website/services/difficulty.service.ts`
- Modify: `website/services/element.service.ts`
- Modify: `website/services/level.service.ts`
- Modify: `website/services/material.service.ts`
- Modify: `website/services/problem.service.ts`
- Modify: `website/services/rank.service.ts`
- Modify: `website/services/ranking.service.ts`
- Modify: `website/services/rarity.service.ts`
- Modify: `website/services/role.service.ts`
- Modify: `website/services/submission.service.ts`
- Modify: `website/services/tag.service.ts`
- Modify: `website/services/user.service.ts`
- Remove after migration: `website/lib/jwt.ts`

**Interfaces:**

```ts
export type RequestOptions<TBody> = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: TBody;
  signal?: AbortSignal;
  timeoutMs?: number;
  idempotencyKey?: string;
  expectedVersion?: number;
};

export async function api<TResponse, TBody = never>(
  path: string,
  options?: RequestOptions<TBody>,
): Promise<TResponse>;
```

- [ ] **Step 1: Write fetch-boundary tests**

Assert same-origin base, `credentials:"include"`, CSRF on unsafe methods, no CSRF on safe methods, JSON content type, timeout/cancellation, no automatic unsafe retry, idempotency/expected-version headers, 204 handling, checked response/error, CID preservation, and no Authorization/localStorage access.

- [ ] **Step 2: Run focused tests**

Run: `cd website && npm run test -- client.test.ts`

Expected: FAIL against current token-based client.

- [ ] **Step 3: Implement the client**

Use native Fetch and `AbortSignal.any`/timeout composition. Read CSRF only from the approved non-HttpOnly bootstrap source. Redact request details in thrown errors. Request IDs use `crypto.randomUUID()` and are reused for one logical unknown-outcome operation until canonical resolution.

- [ ] **Step 4: Migrate services**

Each service exports domain commands/queries with typed DTOs; it does not manage auth, toast, component state, or mock fallback. Delete JWT decoding and token headers once all consumers use session provider.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test -- client.test.ts
rg -n 'localStorage|Authorization|decodeJwt|jwt' app components contexts hooks lib modules services
npm run lint
npm run build
```

Expected: no sensitive browser storage/token decoding match. Theme preference and bounded non-secret coordinator records are the only allowed local/browser storage. Do not commit.

### Task 6: Implement Cross-Tab Refresh Coordination and Canonical Session Provider

**Files:**
- Replace: `website/contexts/auth-context.tsx`
- Create: `website/contexts/session-context.tsx`
- Create: `website/lib/auth/refresh-coordinator.ts`
- Create: `website/lib/auth/pre-session.ts`
- Create: `website/lib/auth/refresh-coordinator.test.ts`
- Create: `website/contexts/session-context.test.tsx`
- Modify: `website/components/app-providers.tsx`
- Modify: `website/components/protected-content.tsx`

**Interfaces:**

```ts
type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "authenticated"; session: SessionView }
  | { status: "error"; error: ApiError };

type SessionContextValue = {
  state: SessionState;
  refresh(): Promise<SessionView | null>;
  logout(): Promise<void>;
  has(capability: string): boolean;
};
```

- [ ] **Step 1: Write concurrency/unknown-outcome tests**

Use fake BroadcastChannel/storage locks to prove one leader refresh, followers wait, same UUID reused after timeout/leader crash/reload within receipt TTL, successful result broadcasts, different key not generated on unknown outcome, bounded record expiry, pre-session survives onboarding reload, and no secret token stored.

- [ ] **Step 2: Run focused tests**

Run:

```bash
cd website
npm run test -- refresh-coordinator.test.ts session-context.test.tsx
```

Expected: FAIL because coordinator/session state does not exist.

- [ ] **Step 3: Implement server-authoritative session**

Bootstrap from `/api/session/me`; never infer login/roles from JWT. On 401, coordinator performs one refresh command with persistent logical UUID, then refetches session. Logout/revoke/suspension clears in-memory state after server confirmation or canonical 401.

- [ ] **Step 4: Implement route capability states**

Protected content distinguishes loading, anonymous→login with safe `next`, authenticated forbidden, and retryable session error. Admin shell uses returned capability keys, not hardcoded role names or local token decode.

- [ ] **Step 5: Verify**

Run:

```bash
cd website
npm run test -- refresh-coordinator.test.ts session-context.test.tsx
npm run lint
npm run build
```

Expected: all exit 0. Do not commit.

## Final Acceptance Gate

After Task 13, re-evaluate every checkbox in **Acceptance Criteria Reference**. The plan exits only when each route, theme, viewport, accessibility, performance, and no-mock item has direct evidence.
