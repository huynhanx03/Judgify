# Register Bundle Reduction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut at least 37,573 gzip bytes from `/register`'s initial JavaScript by eliminating two unnecessary eagerly-loaded libraries — framer-motion (~18 KB) and the @base-ui Dialog module (~17–28 KB) — without affecting the register form's first-paint or accessibility.

**Architecture:** Defer both framer-motion-bearing and Dialog-bearing components out of `/register`'s initial JS with `next/dynamic`, keeping both libraries in the product and in real use. `TraitCard` is deferred inside `trait-selection-section.tsx` (it only renders after a user-triggered roll); `TraitCodexModal` is deferred inside `RegisterFlow.tsx` (it only renders when the codex opens). Both are preloaded ahead of the interaction that needs them. These two changes are independent and touch different files.

**Tech Stack:** Next.js 16 (App Router, RSC), React 19, Tailwind CSS v4, framer-motion (retained, deferred), @base-ui/react (Dialog deferred; Select stays eager), `next/dynamic`.

**LOCKED STACK CONSTRAINT:** framer-motion must NOT be removed from the product. `modules/cultivation/trait-card.tsx` is the only file that imports it and must keep doing so. Replacing it with CSS keyframes is out of scope.

**Spec:** The task brief in the conversation above (no separate file).

## Global Constraints

- Budget gate is 280 × 1024 = 286,720 gzip bytes per public route. Do NOT change `website/scripts/check-bundle.mjs` or the budget constant.
- `npm run verify` must exit 0 (eslint, check:architecture, check:routes, check:user-role-contract, test:contracts, vitest, tsc --noEmit).
- `npm run build && npm run check:bundle` must exit 0.
- No SSR/hydration mismatches; no layout shift; no loading flicker on first render of the register form.
- Accessibility must not regress: labels, keyboard order, focus management, `aria-live` regions, 44 px targets, `prefers-reduced-motion` support.
- All visible text must come from `TEXT` (`@/constants/text`) — no new string literals.
- No git commits, no amends, no file deletions.
- No explanatory comments added unless the file already has them about a non-obvious invariant.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `modules/auth/sections/trait-selection-section.tsx` | Modify | Defer `TraitCard` via `next/dynamic`; preload on roll intent |
| `modules/auth/RegisterFlow.tsx` | Modify | Defer `TraitCodexModal` via `next/dynamic`; preload on mount |

No new files. `modules/cultivation/trait-card.tsx` is NOT modified — framer-motion stays exactly where it is.

## Preload mechanism (applies to both tasks)

In Next.js 16 App Router, `next/dynamic` wraps `React.lazy()` and returns `React.ComponentType<P>` — it does
**not** expose `.preload()`. That method belonged to the pages-router `loadable` implementation; accessing it
fails `tsc --noEmit` and would silently no-op at runtime.

The working idiom is to hold the loader at module scope and call it directly. Webpack caches the chunk promise,
so the later `React.lazy` load resolves from cache with no second network request:

```tsx
const importThing = () => import("@/path/to/thing");
const Thing = dynamic(() => importThing().then((m) => m.Thing), { ssr: false });
function preloadThing() { void importThing(); }
```

## Why deferring these two is safe

- **`TraitCard`** — `TraitSelectionSection` renders unconditionally, but its `TraitCard` instances are gated
  behind `!rolledRootBone && rolledTalents.length === 0`. `traitOffer` starts `null` and is only populated by
  `rollTraits()`, which needs a user click plus a network round-trip. No `TraitCard` is in the first paint on
  either the server or the client. Preloaded on the roll/re-roll buttons' `pointerenter`/`focus` and on
  `isRolling` turning true, so the chunk lands well before the roll response.
- **`TraitCodexModal`** — rendered with `open={showCodex}`, and `showCodex` starts `false`. Preloaded on mount,
  so it is resident long before the user can click the codex button.

---

## Pre-work: Baseline measurement

Before making any changes, record the baseline chunk breakdown so the report section can be filled in accurately.

- [ ] **Baseline: Confirm build artifact exists**

Run:
```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
node -e "const fs=require('fs');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));const reg=r.routes.find(x=>x.route==='/register');console.log('register gzip:', reg.gzipBytes);"
```
Expected: `register gzip: 324293` (or close; confirms the pre-built artifact is the right one).

- [ ] **Baseline: Route-specific chunk breakdown (register vs home)**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
node -e "const fs=require('fs');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));const reg=r.routes.find(x=>x.route==='/register');const home=new Set(r.routes.find(x=>x.route==='/').chunks.map(c=>c.file));let t=0;for(const c of reg.chunks.filter(c=>!home.has(c.file)).sort((a,b)=>b.gzipBytes-a.gzipBytes)){t+=c.gzipBytes;console.log(String(c.gzipBytes).padStart(7),c.file)}console.log('route-specific total',t)"
```
Record output for the final report.

---

## Task 1: Remove framer-motion from TraitCard; replace with CSS entrance animation

**Files:**
- Modify: `modules/cultivation/trait-card.tsx`

**Context:**  
`TraitCard` currently wraps its `full` variant in `<LazyMotion features={loadDomAnimation}><m.button …>` with `initial: {opacity: 0, y: 10}`, `animate: {opacity: 1, y: 0}`, `transition: {type: 'spring', stiffness: 200}`. The `loadDomAnimation` dynamic import brings `framer-motion` into the client-reference-manifest, adding ~18,213 gzip bytes to every route that statically imports `TraitCard`.

The `compact` variant (used in `TraitCodexModal`) does NOT use framer-motion; only the `full` variant (used in `TraitSelectionSection` during registration) does.

**Replacement strategy:**  
Tailwind v4 supports arbitrary keyframes. Add a `@keyframes trait-in` animation via an inline `style` that drives `animation-delay` from the `animationDelay` prop. Use `motion-reduce:` variants so the animation is skipped for users who prefer reduced motion. Drop `useReducedMotion`, `LazyMotion`, `m`, and the `loadDomAnimation` constant entirely.

**Interfaces:**
- Consumes: `TraitCardProps { animationDelay?: number; variant?: "compact" | "full"; ... }` — unchanged.
- Produces: same exported `TraitCard` function signature, same `TraitCardData` type export — unchanged.

- [ ] **Step 1: Confirm framer-motion imports in trait-card.tsx**

Read `modules/cultivation/trait-card.tsx` lines 1–10 to confirm imports to remove:
```
import { LazyMotion, m, useReducedMotion } from "framer-motion";
const loadDomAnimation = () => import("framer-motion").then((mod) => mod.domAnimation);
```

- [ ] **Step 2: Edit trait-card.tsx — remove framer-motion and replace with CSS animation**

Apply the following changes to `/Users/jerry/nhanhmc/coding/Judgify/website/modules/cultivation/trait-card.tsx`:

**Remove** these lines (lines 3–6 in the current file):
```tsx
import { LazyMotion, m, useReducedMotion } from "framer-motion";

const loadDomAnimation = () =>
  import("framer-motion").then((mod) => mod.domAnimation);
```

**Remove** this line inside `TraitCard` function body:
```tsx
  const reduceMotion = useReducedMotion();
```

**Remove** the `motionProps` object (the block starting `const motionProps = {`):
```tsx
  const motionProps = {
    initial: reduceMotion ? false : { opacity: 0, y: 10 },
    animate: { opacity: 1, y: 0 },
    transition: reduceMotion
      ? { duration: 0 }
      : {
          delay: animationDelay,
          type: "spring" as const,
          stiffness: 200,
        },
  };
```

**Add** a `animationStyle` helper just before the `return` of the `full` variant branch:
```tsx
  const animationStyle: React.CSSProperties = animationDelay
    ? { animationDelay: `${animationDelay}s` }
    : {};
```

**Replace** the `full` variant `return` block (the entire `return interactive ? (…) : (…)` that wraps `LazyMotion`):

Old code to replace:
```tsx
  return interactive ? (
    <LazyMotion features={loadDomAnimation}>
      <m.button
        type="button"
        className={className}
        onClick={onClick}
        aria-pressed={selected}
        {...motionProps}
      >
        {content}
      </m.button>
    </LazyMotion>
  ) : (
    <LazyMotion features={loadDomAnimation}>
      <m.div className={className} {...motionProps}>
        {content}
      </m.div>
    </LazyMotion>
  );
```

New code:
```tsx
  return interactive ? (
    <button
      type="button"
      className={cn(className, "animate-trait-in motion-reduce:animate-none")}
      style={animationStyle}
      onClick={onClick}
      aria-pressed={selected}
    >
      {content}
    </button>
  ) : (
    <div
      className={cn(className, "animate-trait-in motion-reduce:animate-none")}
      style={animationStyle}
    >
      {content}
    </div>
  );
```

- [ ] **Step 3: Add animate-trait-in keyframe to global CSS**

The `animate-trait-in` utility must be defined. Open `/Users/jerry/nhanhmc/coding/Judgify/website/app/globals.css` and check if a `@keyframes trait-in` already exists. If not, add the following inside the `@layer utilities` block (or create a new `@layer utilities` block if there isn't one, appending after the last `@layer` block):

```css
@layer utilities {
  @keyframes trait-in {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  .animate-trait-in {
    animation: trait-in 0.35s cubic-bezier(0.34, 1.56, 0.64, 1) both;
  }
}
```

If an `@layer utilities` block already exists, add the keyframe and class inside it.

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npx tsc --noEmit 2>&1 | head -40
```
Expected: zero errors referencing `trait-card.tsx`.

- [ ] **Step 5: Verify ESLint passes on changed file**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npx eslint modules/cultivation/trait-card.tsx
```
Expected: no errors or warnings.

- [ ] **Step 6: Build and measure framer-motion savings**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npm run build 2>&1 | tail -5
```

Then:
```bash
node -e "const fs=require('fs'),z=require('zlib');const c='static/chunks/9176-2fb4ed57381d2819.js';try{const content=fs.readFileSync('.next/'+c,'utf8');console.log('framer chunk still present, size:', z.gzipSync(content,{level:9}).length);}catch(e){console.log('framer chunk gone (expected):',c);}"
```

Then check the register route total:
```bash
node -e "const fs=require('fs');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));const reg=r.routes.find(x=>x.route==='/register');console.log('register after task 1:', reg?.gzipBytes ?? 'run check:bundle first');"
```

Run `npm run check:bundle` and note the new register size. The framer-motion chunk (`9176-...`) should either disappear or shrink significantly.

---

## Task 2: Lazy-load TraitCodexModal with next/dynamic

**Files:**
- Modify: `modules/auth/RegisterFlow.tsx`

**Context:**  
`TraitCodexModal` is rendered at the bottom of `RegisterFlow`'s JSX with `open={showCodex}`. It is never visible at first paint — `showCodex` starts `false`. The modal internally imports `Dialog` from `@/components/ui/dialog`, which wraps `@base-ui/react/dialog`. This pulls in several chunks (including `2928-...` = 16,877 gzip bytes) that are not shared with the `Select` component used by `ProfileAttributeField`. Lazy-loading it removes those Dialog-specific chunks from the RSC client-reference manifest.

**SSR:** The modal must be `ssr: false` because it is a client-only Dialog that relies on DOM APIs. This is consistent with the existing pattern in `components/app-toaster.tsx` and `modules/layout/header.tsx`.

**Preloading:** To avoid a perceived delay when the user clicks the "View Codex" button, preload the lazy chunk on the `mouseenter`/`focus` event of the codex trigger button. This is done by calling the dynamic component's `.preload()` method.

**Interfaces:**
- Consumes: `TraitCodexModal` props — `{ open: boolean; onClose: () => void; traits: TraitResponse[]; status: AsyncResourceStatus; onRetry: () => void }` — unchanged.
- Produces: `RegisterFlow` default export — unchanged.

- [ ] **Step 1: Add the dynamic import at the top of RegisterFlow.tsx**

Read `modules/auth/RegisterFlow.tsx` to confirm the current static import:
```tsx
import { TraitCodexModal } from "@/modules/cultivation/trait-codex-modal";
```

Replace that line with:
```tsx
import dynamic from "next/dynamic";

const TraitCodexModal = dynamic(
  () =>
    import("@/modules/cultivation/trait-codex-modal").then(
      (m) => m.TraitCodexModal,
    ),
  { ssr: false },
);
```

The `dynamic` import from `next/dynamic` should be added with the other imports at the top. The existing `import Link from "next/link"` is already there; add the `dynamic` import right after it (or alongside other `next/*` imports).

- [ ] **Step 2: Add preload on the codex button hover/focus**

Find the codex trigger button in `RegisterFlow`'s JSX (it is inside `TraitSelectionSection`'s `onOpenCodex` prop). The preload must happen in `RegisterFlow` because that is where the `TraitCodexModal` dynamic component is defined.

In `RegisterFlow.tsx`, replace the `onOpenCodex` prop passing:

Old:
```tsx
        onOpenCodex={() => setShowCodex(true)}
```

New — define a handler that preloads before opening:
```tsx
        onOpenCodex={() => {
          TraitCodexModal.preload?.();
          setShowCodex(true);
        }}
```

Also add a preload on the `TraitSelectionSection` container so the chunk loads on first render of that section (the user is likely to click the codex button after rolling). Do this by calling `.preload()` in a `useEffect` that runs once when the component mounts:

In `RegisterFlow.tsx`, add this `useEffect` near the other `useEffect` calls (after the abort controller cleanup effect):

```tsx
  useEffect(() => {
    TraitCodexModal.preload?.();
  }, []);
```

This ensures the dialog chunk is prefetched immediately when the register page loads (without blocking the initial render), so the user experiences no delay when they open the codex.

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npx tsc --noEmit 2>&1 | head -40
```
Expected: zero errors.

- [ ] **Step 4: Verify ESLint passes on changed file**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npx eslint modules/auth/RegisterFlow.tsx
```
Expected: no errors or warnings.

- [ ] **Step 5: Build and measure dialog savings**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npm run build 2>&1 | tail -5
```

Then check register total:
```bash
node -e "const fs=require('fs'),z=require('zlib');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));const reg=r.routes.find(x=>x.route==='/register');console.log('register after task 2:', reg?.gzipBytes);"
```

---

## Task 3: Full verify and final gate check

- [ ] **Step 1: Run the full verify suite**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npm run verify
```
Expected: exits 0 with no errors.

If `tsc --noEmit` fails, read the error carefully. Common causes:
- `TraitCodexModal.preload` — if TypeScript doesn't know about `.preload`, add `?.` optional chaining (already in the plan).
- Missing `React` import in `trait-card.tsx` for `React.CSSProperties` — add `import type React from "react"` if missing (check the existing imports first; the file likely already has `type ReactNode`).

- [ ] **Step 2: Run build**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npm run build
```
Expected: exits 0.

- [ ] **Step 3: Run bundle gate**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
npm run check:bundle
```
Expected: `Frontend bundle budget passed (9 routes, 286720 bytes each)`.

If `/register` still fails, run the route-specific diff:
```bash
node -e "const fs=require('fs');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));const reg=r.routes.find(x=>x.route==='/register');const home=new Set(r.routes.find(x=>x.route==='/').chunks.map(c=>c.file));let t=0;for(const c of reg.chunks.filter(c=>!home.has(c.file)).sort((a,b)=>b.gzipBytes-a.gzipBytes)){t+=c.gzipBytes;console.log(String(c.gzipBytes).padStart(7),c.file)}console.log('route-specific total',t)"
```

If the remaining overage is ≤ 10 KB, the remaining largest register-specific chunk is likely `4230-...` (OnboardingProfileFields + TraitCard rendering code, ~7 KB). That chunk can be reduced by lazy-loading `OnboardingProfileFields` — but only attempt this if the budget still fails after tasks 1 and 2.

- [ ] **Step 4: Collect final per-route numbers for the report**

```bash
cd /Users/jerry/nhanhmc/coding/Judgify/website
node -e "const fs=require('fs');const r=JSON.parse(fs.readFileSync('test-results/bundle-budget.json','utf8'));for(const route of r.routes){console.log(String(route.gzipBytes).padStart(7),route.route);}"
```

Record output. The report back to the user must include:
- Before/after breakdown of route-specific chunks
- What was deferred and why it is safe
- Final gzip number for all 9 routes
- The exact `npm run check:bundle` output line

---

## Contingency: If savings are insufficient after tasks 1 and 2

Estimated savings: ~18,213 (framer-motion) + ~16,877 (Dialog chunk 2928) + possibly additional Dialog-related chunks (7986 ~11,619 if it is Dialog-only). Conservative minimum: 35,090 bytes. Target: 37,573 bytes. The estimate has ~2,500 bytes of margin uncertainty.

**If still over budget by < 5 KB after both tasks:**

Lazy-load `OnboardingProfileFields` in `RegisterFlow.tsx`:

```tsx
import dynamic from "next/dynamic";

const OnboardingProfileFields = dynamic(
  () =>
    import("@/modules/auth/onboarding-profile-fields").then(
      (m) => m.OnboardingProfileFields,
    ),
  {
    ssr: false,
    loading: () => (
      <section
        className="rounded-2xl border border-border/70 bg-card/60 p-4 sm:p-5 mt-4"
        aria-busy="true"
        aria-label={TEXT.ONBOARDING_PROFILE.LOADING}
      >
        <div className="h-4 w-40 animate-pulse rounded bg-muted motion-reduce:animate-none" />
        <div className="mt-3 h-3 w-4/5 animate-pulse rounded bg-muted/80 motion-reduce:animate-none" />
        <div className="mt-5 grid gap-3 sm:grid-cols-2">
          {[0, 1, 2, 3].map((item) => (
            <div
              key={item}
              className="h-28 animate-pulse rounded-2xl border border-border/50 bg-muted/50 motion-reduce:animate-none"
            />
          ))}
        </div>
        <span className="sr-only">{TEXT.ONBOARDING_PROFILE.LOADING}</span>
      </section>
    ),
  },
);
```

WARNING: This defers the profile fields which ARE visible at first paint, so it introduces a brief skeleton flash. Only use this contingency if required to meet the budget and after confirming the skeleton matches the actual component's rendered output to prevent layout shift.

**If still over budget by ≥ 5 KB:**

Something unexpected is pulling in a large shared dependency. Rerun the manifest analysis:
```bash
node -e "const fs=require('fs'),z=require('zlib');const s=fs.readFileSync('.next/server/app/(auth)/register/page_client-reference-manifest.js','utf8');const k='globalThis.__RSC_MANIFEST[\"/(auth)/register/page\"]=';const m=JSON.parse(s.slice(s.indexOf(k)+k.length).trim().replace(/;$/,''));const by=new Map();for(const [id,v] of Object.entries(m.clientModules||{}))for(const c of (v.chunks||[]))if(c.endsWith('.js')){if(!by.has(c))by.set(c,new Set());by.get(c).add(id.split('#')[0])}for(const [c,mods] of [...by].sort((a,b)=>{const sa=z.gzipSync(fs.readFileSync('.next/'+a[0]),{level:9}).length;const sb=z.gzipSync(fs.readFileSync('.next/'+b[0]),{level:9}).length;return sb-sa;})){const size=z.gzipSync(fs.readFileSync('.next/'+c),{level:9}).length;console.log(size,c,'\n  ',[...mods].map(x=>x.replace(/^.*\/website\//,'')).slice(0,8).join(' | '));}"
```
Use the output to identify the new largest culprit and trace its import chain.
